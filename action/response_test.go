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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"zombiezen.com/go/bass/accept"
)

func TestResponseRender(t *testing.T) {
	templateFiles := fstest.MapFS{
		"base.html": {
			Data: []byte("<!DOCTYPE html>\n{{ block \"content\" . }}{{ end }}"),
		},
		"page.html": {
			Data: []byte("{{ define \"content\" }}{{ template \"greet\" . }}, {{ .Subject }}!{{ end }}"),
		},
		"_greet.html": {
			Data: []byte("Hello"),
		},
		"page.txt": {
			Data: []byte("Hello, {{ .Subject }}!\n"),
		},
		"bad.html": {
			Data: []byte("{{ define \"content\" }}bork"), // no {{ end }}
		},
		"stream.html": {
			Data: []byte(`<turbo-stream action="remove" target="{{ .Target }}"></turbo-stream>`),
		},
	}
	tests := []struct {
		name string
		resp *Response
		opts *renderOptions

		wantStatusCode int
		wantHeader     http.Header
		wantBody       string
		ignoreBody     bool
	}{
		{
			name: "Nil",
			resp: nil,
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusNoContent,
			wantHeader:     http.Header{},
		},
		{
			name: "Zero",
			resp: new(Response),
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusNoContent,
			wantHeader:     http.Header{},
		},
		{
			name: "SeeOther",
			resp: &Response{SeeOther: "bar.html"},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/foo/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusSeeOther,
			wantHeader: http.Header{
				"Location": {"/foo/bar.html"},
				// http.Redirect sends a small payload.
				"Content-Type": {"text/html; charset=utf-8"},
			},
			ignoreBody: true,
		},
		{
			name: "PermanentRedirect",
			resp: &Response{
				StatusCode: http.StatusMovedPermanently,
				SeeOther:   "bar.html",
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/foo/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusMovedPermanently,
			wantHeader: http.Header{
				"Location": {"/foo/bar.html"},
				// http.Redirect sends a small payload.
				"Content-Type": {"text/html; charset=utf-8"},
			},
			ignoreBody: true,
		},
		{
			name: "HTMLTemplate",
			resp: &Response{
				HTMLTemplate: "page.html",
				TemplateData: map[string]any{
					"Subject": "World",
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/html; charset=utf-8"},
				"Content-Length":         {"29"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "<!DOCTYPE html>\nHello, World!",
		},
		{
			name: "BadHTMLTemplate",
			resp: &Response{
				HTMLTemplate: "bad.html",
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusInternalServerError,
			wantHeader: http.Header{
				"Content-Type":           {"text/plain; charset=utf-8"},
				"X-Content-Type-Options": {"nosniff"},
			},
			ignoreBody: true,
		},
		{
			name: "HTMLTemplateFilesMissing",
			resp: &Response{
				HTMLTemplate: "page.html",
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: nil,
			},
			wantStatusCode: http.StatusInternalServerError,
			wantHeader: http.Header{
				"Content-Type":           {"text/plain; charset=utf-8"},
				"X-Content-Type-Options": {"nosniff"},
			},
			ignoreBody: true,
		},
		{
			name: "TurboStreamTemplate",
			resp: &Response{
				TurboStreamTemplate: "stream.html",
				TemplateData: map[string]any{
					"Target": "junk",
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/vnd.turbo-stream.html; charset=utf-8"},
				"Content-Length":         {"59"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: `<turbo-stream action="remove" target="junk"></turbo-stream>`,
		},
		{
			name: "JSON",
			resp: &Response{
				JSONValue: map[string]any{"greeting": "hello world"},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"application/json; charset=utf-8"},
				"Content-Length":         {"26"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: `{"greeting":"hello world"}`,
		},
		{
			name: "PlainTextTemplate",
			resp: &Response{
				TemplateData: map[string]any{"Subject": "World"},
				TextTemplate: "page.txt",
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/plain; charset=utf-8"},
				"Content-Length":         {"14"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "Hello, World!\n",
		},
		{
			name: "PlainText",
			resp: &Response{
				Other: []*Representation{TextRepresentation("Hello, World!\n")},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/plain; charset=utf-8"},
				"Content-Length":         {"14"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "Hello, World!\n",
		},
		{
			name: "HTMLAndText/Equal",
			resp: &Response{
				HTMLTemplate: "page.html",
				TextTemplate: "page.txt",
				TemplateData: map[string]any{
					"Subject": "World",
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/html; charset=utf-8"},
				"Content-Length":         {"29"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "<!DOCTYPE html>\nHello, World!",
		},
		{
			name: "HTMLAndText/ClientPrefersText",
			resp: &Response{
				HTMLTemplate: "page.html",
				TextTemplate: "page.txt",
				TemplateData: map[string]any{
					"Subject": "World",
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "text/html", Quality: 0.9},
					{Range: "text/plain", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/plain; charset=utf-8"},
				"Content-Length":         {"14"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "Hello, World!\n",
		},
		{
			name: "HTMLAndText/ClientPrefersRandom",
			resp: &Response{
				HTMLTemplate: "page.html",
				TextTemplate: "page.txt",
				TemplateData: map[string]any{
					"Subject": "World",
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "application/foo", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/html; charset=utf-8"},
				"Content-Length":         {"29"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "<!DOCTYPE html>\nHello, World!",
		},
		{
			name: "HTMLAndText/ClientPrefersOther",
			resp: &Response{
				HTMLTemplate: "page.html",
				TextTemplate: "page.txt",
				TemplateData: map[string]any{
					"Subject": "World",
				},
				Other: []*Representation{
					{
						Header: http.Header{
							"Content-Type":   {"text/csv"},
							"Content-Length": {"13"},
						},
						Body: io.NopCloser(strings.NewReader("Hello,World\r\n")),
					},
				},
			},
			opts: &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/",
				acceptHeader: accept.Header{
					{Range: "text/csv", Quality: 1.0},
				},
				templateFiles: templateFiles,
			},
			wantStatusCode: http.StatusOK,
			wantHeader: http.Header{
				"Content-Type":           {"text/csv"},
				"Content-Length":         {"13"},
				"X-Content-Type-Options": {"nosniff"},
			},
			wantBody: "Hello,World\r\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			rec := httptest.NewRecorder()
			test.opts.reportError = func(ctx context.Context, err error) {
				t.Log("Render reported error:", err)
			}
			test.resp.render(ctx, rec, test.opts)

			got := rec.Result()
			defer got.Body.Close()
			gotBody, err := readAllString(got.Body)
			if err != nil {
				t.Error("Reading body:", err)
			}
			if got.StatusCode != test.wantStatusCode {
				t.Errorf("StatusCode = %d; want %d", got.StatusCode, test.wantStatusCode)
			}
			if test.ignoreBody {
				got.Header.Del("Content-Length")
			}
			if diff := cmp.Diff(test.wantHeader, got.Header, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Header (-want +got):\n%s", diff)
			}
			if !test.ignoreBody {
				if diff := cmp.Diff(test.wantBody, gotBody); diff != "" {
					t.Errorf("Body (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestForceAccept(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		const want = "application/foo, */*;q=0.9"
		var got []string
		srv := httptest.NewServer(&ForceAccept{
			Accept: want,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header["Accept"]
				w.WriteHeader(http.StatusNoContent)
			}),
		})
		t.Cleanup(srv.Close)
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "blargle")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if !cmp.Equal([]string{want}, got) {
			t.Errorf("Accept = %q; want [%q]", got, want)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		var got []string
		srv := httptest.NewServer(&ForceAccept{
			Accept: "",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header["Accept"]
				w.WriteHeader(http.StatusNoContent)
			}),
		})
		t.Cleanup(srv.Close)
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "blargle")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(got) != 0 {
			t.Errorf("Accept = %q; want []", got)
		}
	})
}

func readAllString(r io.Reader) (string, error) {
	sb := new(strings.Builder)
	_, err := io.Copy(sb, r)
	return sb.String(), err
}
