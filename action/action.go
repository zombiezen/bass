// Copyright 2022 The Bass Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package action

import (
	"context"
	"errors"
	"html/template"
	"io/fs"
	"net/http"

	"zombiezen.com/go/bass/accept"
)

const acceptHeaderName = "Accept"

type Func[R any] func(context.Context, R) (*Response, error)

// A Handler responds to an HTTP request by calling a [Func].
type Handler[R any] struct {
	f   Func[R]
	cfg Config[R]
}

// NewHandler returns a new [Handler] with a default [Config] that calls f.
func NewHandler(templateFiles fs.FS, f Func[*http.Request]) *Handler[*http.Request] {
	cfg := &Config[*http.Request]{
		TransformRequest: identity,
		TemplateFiles:    templateFiles,
	}
	return cfg.NewHandler(f)
}

// ServeHTTP handles an HTTP request.
func (h *Handler[R]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.cfg.MaxRequestSize > 0 {
		r = r.Clone(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxRequestSize)
	}
	resp, renderOpts, err := h.serve(r)
	defer func() {
		if err := resp.Close(); err != nil {
			h.cfg.reportError(ctx, err)
		}
	}()
	if err != nil {
		h.cfg.reportError(ctx, err)
		if resp == nil {
			resp = h.cfg.transformError(err)
		}
	}
	resp.render(ctx, w, renderOpts)
}

func (h *Handler[R]) serve(r *http.Request) (*Response, *renderOptions, error) {
	ctx := r.Context()
	renderOpts := &renderOptions{
		reqMethod:     r.Method,
		reqPath:       r.URL.Path,
		templateFiles: h.cfg.TemplateFiles,
		reportError:   h.cfg.ReportError,
	}
	var err error
	renderOpts.acceptHeader, err = accept.ParseHeader(r.Header.Get(acceptHeaderName))
	if err != nil {
		renderOpts.templateFuncs = h.cfg.TemplateFuncs
		return nil, renderOpts, WithStatusCode(http.StatusBadRequest, err)
	}
	req, cleanup, err := h.cfg.transformRequest(r)
	if err != nil {
		renderOpts.templateFuncs = h.cfg.TemplateFuncs
		return nil, renderOpts, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	// TODO(maybe): Randomize order of f and MakeTemplateFuncs.
	resp, err := h.f(ctx, req)
	if h.cfg.MakeRequestTemplateFuncs != nil && (err == nil || resp != nil) {
		// Only set template functions if we are not using transformError.
		// This keeps transformError robust because it cannot ever observe request-specific functions.
		renderOpts.templateFuncs = make(template.FuncMap)
		for name, f := range h.cfg.TemplateFuncs {
			renderOpts.templateFuncs[name] = f
		}
		for name, f := range h.cfg.MakeRequestTemplateFuncs(ctx, req) {
			renderOpts.templateFuncs[name] = f
		}
	} else {
		renderOpts.templateFuncs = h.cfg.TemplateFuncs
	}
	return resp, renderOpts, err
}

// A Config contains options for creating a [Handler].
// The Config type is parameterized on request type.
type Config[R any] struct {
	// TransformRequest converts an [*http.Request] to the [Func]'s request type.
	// If R is *http.Request and TransformRequest is nil,
	// then the *http.Request is used verbatim.
	// Otherwise, if TransformRequest is nil, the Handler will serve errors.
	//
	// TransformRequest may return a cleanup function,
	// which will be called after the [Func] is called
	// to release any resources associated with the returned request.
	//
	// If TransformRequest returns an error that is not given a status code with [WithStatusCode],
	// then 400 (Bad Request) is assumed.
	TransformRequest func(*http.Request) (request R, cleanup func(), err error)

	// If MaxRequestSize is greater than zero,
	// then Handler will place an [http.MaxBytesReader] on the request body
	// before it is sent to TransformRequest.
	MaxRequestSize int64

	// TransformError is an optional callback to convert errors into responses.
	// If nil, a basic plain text conversion will be performed
	// that uses the status code from [ErrorStatusCode].
	//
	// Templated error responses can only use funcs from TemplateFuncs,
	// not MakeRequestTemplateFuncs,
	// because TransformError may be called prior to TransformRequest
	// in case of a bad request.
	TransformError func(error) *Response

	// TemplateFiles is used for reading templates for responses.
	// It is only needed if the handler uses the template fields in [Response].
	TemplateFiles fs.FS

	// TemplateFuncs is a set of functions available in every response.
	TemplateFuncs template.FuncMap

	// MakeRequestTemplateFuncs is a callback that produces a set of functions
	// available in responses returned from the handler's [Func].
	MakeRequestTemplateFuncs func(context.Context, R) template.FuncMap

	// ReportError is an optional callback
	// for application errors that occur during request processing.
	ReportError func(context.Context, error)
}

// NewHandler creates a [Handler] with the given function.
func (cfg *Config[R]) NewHandler(f Func[R]) *Handler[R] {
	if cfg == nil {
		cfg = new(Config[R])
	}
	return &Handler[R]{f, *cfg}
}

var errNoFunc = errors.New("TransformRequest function not provided")

func (cfg *Config[R]) transformRequest(r *http.Request) (req R, cleanup func(), err error) {
	if cfg == nil || cfg.TransformRequest == nil {
		var zero R
		if _, ok := any(zero).(*http.Request); !ok {
			return zero, nil, errNoFunc
		}
		var req2 *http.Request
		req2, cleanup, err = identity(r)
		req = any(req2).(R)
	} else {
		req, cleanup, err = cfg.TransformRequest(r)
	}
	if err != nil {
		if _, codeSet := errorStatusCode(err); !codeSet {
			err = WithStatusCode(http.StatusBadRequest, err)
		}
	}
	return
}

func (cfg *Config[R]) transformError(err error) *Response {
	if cfg == nil || cfg.TransformError == nil {
		return defaultTransformError(err)
	}
	return cfg.TransformError(err)
}

func (cfg *Config[R]) reportError(ctx context.Context, err error) {
	if cfg != nil && cfg.ReportError != nil {
		cfg.ReportError(ctx, err)
	}
}

func identity(r *http.Request) (*http.Request, func(), error) {
	return r, func() {}, nil
}
