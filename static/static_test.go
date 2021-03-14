// Copyright 2021 The Bass Authors
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

package static

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	const host = "example.com"
	fsys := fstest.MapFS{
		"foo.txt": {
			Data: []byte("Hello, World!\n"),
		},
		"dir/bar.txt": {
			Data: []byte("bar"),
		},
		"dir/baz.txt": {
			Data: []byte("baz"),
		},
	}
	tests := []struct {
		name string
		req  *http.Request

		wantStatusCode int
		wantBody       func(string) bool
		wantLocation   string
	}{
		{
			name: "FoundFile",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/foo.txt",
				},
			},
			wantStatusCode: http.StatusOK,
			wantBody:       func(s string) bool { return s == "Hello, World!\n" },
		},
		{
			name: "NotFound",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/404.txt",
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "Post",
			req: &http.Request{
				Method: http.MethodPost,
				Host:   host,
				URL: &url.URL{
					Path: "/foo.txt",
				},
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "Root",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/",
				},
			},
			wantStatusCode: http.StatusOK,
			wantBody: func(s string) bool {
				return strings.Contains(s, "foo.txt") && strings.Contains(s, "dir")
			},
		},
		{
			name: "Subdir",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/dir/",
				},
			},
			wantStatusCode: http.StatusOK,
			wantBody: func(s string) bool {
				return strings.Contains(s, "bar.txt") && strings.Contains(s, "baz.txt")
			},
		},
		{
			name: "SubdirWithoutSlash",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/dir",
				},
			},
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "dir/",
		},
		{
			name: "FileURLWithSlash",
			req: &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: "/foo.txt/",
				},
			},
			wantStatusCode: http.StatusMovedPermanently,
			wantLocation:   "../foo.txt",
		},
	}
	h := NewHandler(fsys)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, test.req)
			got := rec.Result()
			defer got.Body.Close()
			if got.StatusCode != test.wantStatusCode {
				t.Errorf("got HTTP %d; want %d", got.StatusCode, test.wantStatusCode)
			}
			if gotLocation := got.Header.Get("Location"); test.wantLocation != "" && gotLocation != test.wantLocation {
				t.Errorf("Location = %q; want %q", gotLocation, test.wantLocation)
			}

			if test.wantBody != nil {
				gotData, err := io.ReadAll(got.Body)
				if err != nil {
					t.Error("Read body:", err)
				}
				if !test.wantBody(string(gotData)) {
					t.Errorf("body = %q; did not match expectations", gotData)
				}
			}
		})
	}

	t.Run("ETag", func(t *testing.T) {
		const path = "/foo.txt"
		rec1 := httptest.NewRecorder()
		h.ServeHTTP(rec1, &http.Request{
			Method: http.MethodGet,
			Host:   host,
			URL: &url.URL{
				Path: path,
			},
		})
		got1 := rec1.Result()
		got1.Body.Close()
		if want := http.StatusOK; got1.StatusCode != want {
			t.Fatalf("got HTTP %d; want %d", got1.StatusCode, want)
		}
		etag := got1.Header.Get("ETag")
		if etag == "" {
			t.Fatal("ETag not set")
		}

		t.Run("Match", func(t *testing.T) {
			rec2 := httptest.NewRecorder()
			h.ServeHTTP(rec2, &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: path,
				},
				Header: http.Header{
					http.CanonicalHeaderKey("If-None-Match"): {etag},
				},
			})
			got2 := rec2.Result()
			got2.Body.Close()
			if want := http.StatusNotModified; got2.StatusCode != want {
				t.Errorf("got HTTP %d; want %d", got2.StatusCode, want)
			}
		})

		t.Run("NoneMatch", func(t *testing.T) {
			rec2 := httptest.NewRecorder()
			h.ServeHTTP(rec2, &http.Request{
				Method: http.MethodGet,
				Host:   host,
				URL: &url.URL{
					Path: path,
				},
				Header: http.Header{
					http.CanonicalHeaderKey("If-None-Match"): {`"xyzzy"`},
				},
			})
			got2 := rec2.Result()
			got2.Body.Close()
			if want := http.StatusOK; got2.StatusCode != want {
				t.Errorf("got HTTP %d; want %d", got2.StatusCode, want)
			}
		})
	})
}
