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
	"errors"
	"net/http"
)

// ErrNotFound is a generic "not found" error
// that can be returned from a [Func] to render an HTTP 404 (Not Found) response.
var ErrNotFound = WithStatusCode(http.StatusNotFound, errors.New("404 not found"))

type httpError struct {
	code int
	err  error
}

// WithStatusCode returns an error for which [ErrorStatusCode] returns the given code
// and [errors.Unwrap] returns the given error.
func WithStatusCode(code int, err error) error {
	return httpError{code, err}
}

func (e httpError) Error() string {
	return e.err.Error()
}

func (e httpError) Unwrap() error {
	return e.err
}

// ErrorStatusCode finds the first error in err's chain that was created by [WithStatusCode],
// and if one is found, returns the HTTP status code.
// If err is nil, it returns 200 (OK).
// Otherwise, it returns 500 (Internal Server Error).
func ErrorStatusCode(err error) int {
	code, _ := errorStatusCode(err)
	return code
}

func errorStatusCode(err error) (code int, explicit bool) {
	if err == nil {
		return http.StatusOK, false
	}
	var e httpError
	if !errors.As(err, &e) {
		return http.StatusInternalServerError, false
	}
	return e.code, true
}

func defaultTransformError(err error) *Response {
	return &Response{
		StatusCode: ErrorStatusCode(err),
		Other: []*Representation{
			TextRepresentation(err.Error()),
		},
	}
}
