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

package graphqlhttp

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string

		method      string
		query       url.Values
		contentType string
		body        string

		want          *Request
		wantErrStatus int
	}{
		{
			name:   "HEAD",
			method: http.MethodHead,
			query:  url.Values{"query": {"{me{name}}"}},
			want: &Request{
				Query: "{me{name}}",
			},
		},
		{
			name:   "GET/JustQuery",
			method: http.MethodGet,
			query:  url.Values{"query": {"{me{name}}"}},
			want: &Request{
				Query: "{me{name}}",
			},
		},
		{
			name:   "GET/AllFields",
			method: http.MethodGet,
			query: url.Values{
				"query":         {"{me{name}}"},
				"variables":     {`{"foo":"bar"}`},
				"operationName": {"Baz"},
			},
			want: &Request{
				Query:         "{me{name}}",
				OperationName: "Baz",
				Variables:     map[string]interface{}{"foo": "bar"},
			},
		},
		{
			name:        "POST/JustQuery",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{me{name}}"}`,
			want: &Request{
				Query: "{me{name}}",
			},
		},
		{
			name:        "POST/AllFields",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{me{name}}", "variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: &Request{
				Query:         "{me{name}}",
				OperationName: "Baz",
				Variables:     map[string]interface{}{"foo": "bar"},
			},
		},
		{
			name:        "POST/QueryInURL",
			method:      http.MethodPost,
			query:       url.Values{"query": {"{me{name}}"}},
			contentType: "application/json; charset=utf-8",
			body:        `{"variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: &Request{
				Query:         "{me{name}}",
				OperationName: "Baz",
				Variables:     map[string]interface{}{"foo": "bar"},
			},
		},
		{
			name:        "POST/QueryInBodyAndURL",
			method:      http.MethodPost,
			query:       url.Values{"query": {"{me{name}}"}},
			contentType: "application/json; charset=utf-8",
			body:        `{"query": "{your{face}}", "variables": {"foo":"bar"}, "operationName": "Baz"}`,
			want: &Request{
				Query:         "{your{face}}",
				OperationName: "Baz",
				Variables:     map[string]interface{}{"foo": "bar"},
			},
		},
		{
			name:        "POST/GraphQLContentType",
			method:      http.MethodPost,
			contentType: "application/graphql; charset=utf-8",
			body:        "{me{name}}",
			want: &Request{
				Query: "{me{name}}",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &http.Request{
				Method: test.method,
				URL: &url.URL{
					RawQuery: test.query.Encode(),
				},
				Header: make(http.Header),
				Body:   ioutil.NopCloser(strings.NewReader(test.body)),
			}
			if test.contentType != "" {
				req.Header.Set("Content-Type", test.contentType)
			}
			got, err := Parse(req)
			if err != nil {
				if test.wantErrStatus == 0 {
					t.Fatalf("Parse error = %v; want <nil>", err)
				}
				if StatusCode(err) != test.wantErrStatus {
					t.Fatalf("Parse error = %v, status code = %d; want status code = %d", err, StatusCode(err), test.wantErrStatus)
				}
				return
			}
			if test.wantErrStatus != 0 {
				t.Fatalf("Parse(...) = %+v, <nil>; want error status code = %d", got, test.wantErrStatus)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Parse(...) (-want +got):\n%s", diff)
			}
		})
	}
}
