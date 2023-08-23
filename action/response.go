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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	texttemplate "text/template"

	"zombiezen.com/go/bass/accept"
	"zombiezen.com/go/bass/templateloader"
	"zombiezen.com/go/bass/turbostream"
)

const (
	contentTypeHeaderName        = "Content-Type"
	contentTypeOptionsHeaderName = "X-Content-Type-Options"
	contentLengthHeaderName      = "Content-Length"
)

const (
	htmlType  = "text/html"
	plainType = "text/plain"
	jsonType  = "application/json"
)

const charsetUTF8Params = "; charset=utf-8"

// Response represents an HTTP response.
// It contains zero or more representations of its resource,
// which will be selected via [content negotiation].
// A nil or zero Response represents an HTTP 204 (No Content) response.
//
// [content negotiation]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Content_negotiation
type Response struct {
	// StatusCode is the response's HTTP status code.
	// If it is zero and SeeOther is not empty, then 303 (See Other) is assumed,
	// otherwise 200 (OK).
	StatusCode int
	// SeeOther specifies the response's [Location header].
	// If it is not empty, then the response is a redirect.
	//
	// [Location header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location
	SeeOther string

	// SetCookies is a list of cookies to add as Set-Cookie headers.
	// The provided cookies must have valid names.
	// Invalid cookies may be silent dropped.
	SetCookies []*http.Cookie

	// TemplateData is passed to the templates.
	// See [text/template] for details.
	TemplateData any
	// HTMLTemplate names an html/template file to use to present HTML.
	HTMLTemplate string
	// TurboStreamTemplate names an html/template file to use to present Turbo Stream data.
	TurboStreamTemplate string
	// TextTemplate names a text/template file to use to present plain text.
	TextTemplate string
	// JSONValue is a value to marshal to present JSON.
	JSONValue any

	// Other lists representations of the response.
	Other []*Representation
}

// IsEmpty reports whether the response is nil
// or does not have any valid representations.
func (resp *Response) IsEmpty() bool {
	if resp == nil {
		return true
	}
	if resp.HTMLTemplate != "" ||
		resp.TurboStreamTemplate != "" ||
		resp.TextTemplate != "" ||
		resp.JSONValue != nil {
		return false
	}
	for _, repr := range resp.Other {
		if _, _, err := mime.ParseMediaType(repr.Header.Get(contentTypeHeaderName)); err == nil {
			return false
		}
	}
	return true
}

// IsRedirect reports whether resp.SeeOther is not empty.
func (resp *Response) IsRedirect() bool {
	return resp != nil && resp.SeeOther != ""
}

// Close closes the bodies of all representations,
// returning the first error encountered.
func (resp *Response) Close() error {
	if resp == nil {
		return nil
	}
	var first error
	for _, repr := range resp.Other {
		if repr.Body != nil {
			if err := repr.Body.Close(); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

// A Representation is a representation of a [Response].
// The Header must contain a value for Content-Type for it to be used.
type Representation struct {
	Header http.Header
	Body   io.ReadCloser
}

// TextRepresentation creates a plain text representation of a string.
func TextRepresentation(s string) *Representation {
	return &Representation{
		Header: http.Header{
			contentTypeHeaderName:   {plainType + charsetUTF8Params},
			contentLengthHeaderName: {strconv.Itoa(len(s))},
		},
		Body: io.NopCloser(strings.NewReader(s)),
	}
}

// Write copies the representation to the response writer.
func (repr *Representation) Write(w http.ResponseWriter, code int) error {
	return repr.write(w, code, false)
}

func (repr *Representation) write(w http.ResponseWriter, code int, head bool) error {
	if repr.Header.Get(contentTypeHeaderName) == "" {
		return fmt.Errorf("write representation: does not have a %s header", contentTypeHeaderName)
	}
	h := w.Header()
	for k, v := range repr.Header {
		h[k] = append(h[k], v...)
	}
	if len(h[contentTypeOptionsHeaderName]) == 0 {
		h.Set(contentTypeOptionsHeaderName, "nosniff")
	}
	w.WriteHeader(code)
	if !head {
		return nil
	}
	_, err := io.Copy(w, repr.Body)
	return err
}

type renderOptions struct {
	reqMethod    string
	reqPath      string
	acceptHeader accept.Header

	templateFiles fs.FS
	templateFuncs template.FuncMap
	reportError   func(context.Context, error)
}

func (resp *Response) render(ctx context.Context, w http.ResponseWriter, opts *renderOptions) {
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	for _, cookie := range resp.SetCookies {
		http.SetCookie(w, cookie)
	}
	if resp.SeeOther != "" {
		statusCode := http.StatusSeeOther
		if resp.StatusCode != 0 {
			statusCode = resp.StatusCode
		}
		fakeReq := &http.Request{
			Method: opts.reqMethod,
			URL:    &url.URL{Path: opts.reqPath},
		}
		http.Redirect(w, fakeReq, resp.SeeOther, statusCode)
		return
	}
	possibilities := resp.gatherRepresentations(func(err error) {
		if opts.reportError != nil {
			opts.reportError(ctx, err)
		}
	})
	if len(possibilities) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p := preferredRepresentation(possibilities, opts.acceptHeader)
	repr := p.repr
	if repr == nil {
		var err error
		repr, err = p.reprFunc(opts)
		if err != nil {
			if opts.reportError != nil {
				opts.reportError(ctx, err)
			}
			http.Error(w, "Error while serving page. Check server logs.", http.StatusInternalServerError)
			return
		}
	}
	code := resp.StatusCode
	if code == 0 {
		code = http.StatusOK
	}
	repr.write(w, code, opts.reqMethod != http.MethodHead)
}

type parsedRepresentation struct {
	contentType string
	mediaType   string
	typeParams  map[string]string
	repr        *Representation
	reprFunc    func(*renderOptions) (*Representation, error)
}

func (resp *Response) gatherRepresentations(report func(error)) []parsedRepresentation {
	possibilities := make([]parsedRepresentation, 0, 4+len(resp.Other))
	utf8Params := map[string]string{"charset": "utf-8"}
	if resp.TurboStreamTemplate != "" {
		possibilities = append(possibilities, parsedRepresentation{
			contentType: turbostream.ContentType + charsetUTF8Params,
			mediaType:   turbostream.ContentType,
			typeParams:  utf8Params,
			reprFunc:    resp.turboStreamRepresentation,
		})
	}
	if resp.HTMLTemplate != "" {
		possibilities = append(possibilities, parsedRepresentation{
			contentType: htmlType + charsetUTF8Params,
			mediaType:   htmlType,
			typeParams:  utf8Params,
			reprFunc:    resp.htmlRepresentation,
		})
	}
	if resp.JSONValue != nil {
		possibilities = append(possibilities, parsedRepresentation{
			contentType: jsonType + charsetUTF8Params,
			mediaType:   jsonType,
			typeParams:  utf8Params,
			reprFunc:    resp.jsonRepresentation,
		})
	}
	if resp.TextTemplate != "" {
		possibilities = append(possibilities, parsedRepresentation{
			contentType: plainType + charsetUTF8Params,
			mediaType:   plainType,
			typeParams:  utf8Params,
			reprFunc:    resp.textRepresentation,
		})
	}
	for _, repr := range resp.Other {
		contentType := repr.Header.Get(contentTypeHeaderName)
		mediaType, typeParams, err := mime.ParseMediaType(contentType)
		if err != nil {
			report(fmt.Errorf("invalid %s on representation (skipping): %v", contentTypeHeaderName, err))
			continue
		}
		possibilities = append(possibilities, parsedRepresentation{
			contentType: contentType,
			mediaType:   mediaType,
			typeParams:  typeParams,
			repr:        repr,
		})
	}
	return possibilities
}

// preferredRepresentation returns the user's most preferred representation from the list,
// using representations earlier in the list in case of a tie.
func preferredRepresentation(possibilities []parsedRepresentation, acceptHeader accept.Header) *parsedRepresentation {
	if len(possibilities) == 0 {
		return nil
	}
	p := &possibilities[0]
	q := acceptHeader.Quality(p.mediaType, p.typeParams)
	for i := range possibilities[1:] {
		pi := &possibilities[1+i]
		qi := acceptHeader.Quality(pi.mediaType, pi.typeParams)
		if qi > q {
			p, q = pi, qi
		}
	}
	return p
}

func (resp *Response) htmlRepresentation(opts *renderOptions) (*Representation, error) {
	if opts.templateFiles == nil {
		return nil, errNoTemplateFiles
	}
	base, err := templateloader.Base(opts.templateFiles, opts.templateFuncs)
	if err != nil {
		return nil, err
	}
	tmpl, err := templateloader.Extend(base, opts.templateFiles, resp.HTMLTemplate)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, resp.TemplateData)
	if err != nil {
		return nil, err
	}
	return &Representation{
		Header: http.Header{
			contentTypeHeaderName:   {htmlType + charsetUTF8Params},
			contentLengthHeaderName: {strconv.Itoa(buf.Len())},
		},
		Body: io.NopCloser(buf),
	}, nil
}

func (resp *Response) turboStreamRepresentation(opts *renderOptions) (*Representation, error) {
	if opts.templateFiles == nil {
		return nil, errNoTemplateFiles
	}
	tmpl, err := templateloader.ParseFile(
		template.New(resp.TurboStreamTemplate).Funcs(opts.templateFuncs),
		opts.templateFiles,
		resp.TurboStreamTemplate,
	)
	if err != nil {
		return nil, err
	}
	if _, err := templateloader.AddPartials(tmpl, opts.templateFiles); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, resp.TemplateData)
	if err != nil {
		return nil, err
	}
	return &Representation{
		Header: http.Header{
			contentTypeHeaderName:   {turbostream.ContentType + charsetUTF8Params},
			contentLengthHeaderName: {strconv.Itoa(buf.Len())},
		},
		Body: io.NopCloser(buf),
	}, nil
}

func (resp *Response) jsonRepresentation(opts *renderOptions) (*Representation, error) {
	jsonData, err := json.Marshal(resp.JSONValue)
	if err != nil {
		return nil, err
	}
	return &Representation{
		Header: http.Header{
			contentTypeHeaderName:   {jsonType + charsetUTF8Params},
			contentLengthHeaderName: {strconv.Itoa(len(jsonData))},
		},
		Body: io.NopCloser(bytes.NewReader(jsonData)),
	}, nil
}

func (resp *Response) textRepresentation(opts *renderOptions) (*Representation, error) {
	if opts.templateFiles == nil {
		return nil, errNoTemplateFiles
	}
	tmpl, err := templateloader.ParseTextFile(
		texttemplate.New(resp.TextTemplate).Funcs(texttemplate.FuncMap(opts.templateFuncs)),
		opts.templateFiles,
		resp.TextTemplate,
	)
	if err != nil {
		return nil, err
	}
	if _, err := templateloader.AddTextPartials(tmpl, opts.templateFiles); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, resp.TemplateData)
	if err != nil {
		return nil, err
	}
	return &Representation{
		Header: http.Header{
			contentTypeHeaderName:   {plainType + charsetUTF8Params},
			contentLengthHeaderName: {strconv.Itoa(buf.Len())},
		},
		Body: io.NopCloser(buf),
	}, nil
}

var errNoTemplateFiles = errors.New("render: TemplateFiles missing from Handler")

// ForceAccept is an HTTP middleware
// that unconditionally sets the Accept request header
// to a given string.
type ForceAccept struct {
	Accept  string
	Handler http.Handler
}

// ForceJSON wraps a handler with a [ForceAccept] with a JSON content type.
func ForceJSON(h http.Handler) *ForceAccept {
	return &ForceAccept{
		Accept:  jsonType,
		Handler: h,
	}
}

// ServeHTTP calls fa.Handler.ServeHTTP with a copy of r
// where the Accept header is set to fa.Accept.
func (fa *ForceAccept) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hdr := r.Header.Clone()
	if fa.Accept != "" {
		hdr.Set(acceptHeaderName, fa.Accept)
	} else {
		hdr.Del(acceptHeaderName)
	}
	r = r.Clone(r.Context())
	r.Header = hdr
	fa.Handler.ServeHTTP(w, r)
}
