// Copyright 2019 The Bass Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package graphqlhttp provides functions for serving GraphQL over HTTP as
// described in https://graphql.org/learn/serving-over-http/.
package graphqlhttp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"

	"github.com/graphql-go/graphql"
	"golang.org/x/xerrors"
)

// Request is a decoded GraphQL HTTP request.
type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// Parse parses a GraphQL HTTP request. If an error is returned, StatusCode
// will return the proper HTTP status code to use.
//
// Request methods may be GET, HEAD, or POST. If the method is not one of these,
// then an error is returned that will make StatusCode return
// http.StatusMethodNotAllowed.
func Parse(r *http.Request) (*Request, error) {
	request := &Request{
		Query: r.URL.Query().Get("query"),
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if v := r.FormValue("variables"); v != "" {
			if err := json.Unmarshal([]byte(v), &request.Variables); err != nil {
				return nil, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
		}
		request.OperationName = r.FormValue("operationName")
	case http.MethodPost:
		rawContentType := r.Header.Get("Content-Type")
		contentType, _, err := mime.ParseMediaType(rawContentType)
		if err != nil {
			return nil, &httpError{
				msg:  "parse graphql request: invalid content type: " + rawContentType,
				code: http.StatusUnsupportedMediaType,
			}
		}
		switch contentType {
		case "application/json":
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				return nil, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
		case "application/x-www-form-urlencoded":
			request.Query = r.FormValue("query")
		case "application/graphql":
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return nil, &httpError{
					msg:   "parse graphql request: ",
					code:  http.StatusBadRequest,
					cause: err,
				}
			}
			if len(data) > 0 {
				request.Query = string(data)
			}
		default:
			return nil, &httpError{
				msg:  "parse graphql request: unrecognized content type: " + contentType,
				code: http.StatusUnsupportedMediaType,
			}
		}
	default:
		return nil, &httpError{
			msg:  fmt.Sprintf("parse graphql request: method %s not allowed", r.Method),
			code: http.StatusMethodNotAllowed,
		}
	}
	return request, nil
}

type httpError struct {
	msg   string
	code  int
	cause error
}

func (e *httpError) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + e.cause.Error()
}

func (e *httpError) Unwrap() error {
	return e.cause
}

// StatusCode returns the HTTP status code an error indicates.
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var e *httpError
	if !xerrors.As(err, &e) {
		return http.StatusInternalServerError
	}
	return e.code
}

// WriteResponse writes a GraphQL result as an HTTP response.
func WriteResponse(w http.ResponseWriter, result *graphql.Result) {
	payload, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "GraphQL marshal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if _, err := w.Write(payload); err != nil {
		return
	}
}
