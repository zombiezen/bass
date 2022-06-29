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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	t.Run("HTMLResponse", func(t *testing.T) {
		templateFiles := fstest.MapFS{
			"base.html": {
				Data: []byte("<!DOCTYPE html>\n{{ block \"content\" . }}{{ end }}"),
			},
			"page.html": {
				Data: []byte("{{ define \"content\" }}Hello, {{ .Subject }}!{{ end }}"),
			},
		}
		h := NewHandler(templateFiles, func(ctx context.Context, r *http.Request) (*Response, error) {
			return &Response{
				HTMLTemplate: "page.html",
				TemplateData: map[string]any{"Subject": "World"},
			}, nil
		})
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		resp, err := srv.Client().Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("StatusCode = %d; want %d", got, want)
		}
		got, err := readAllString(resp.Body)
		if err != nil {
			t.Error(err)
		}
		const want = "<!DOCTYPE html>\nHello, World!"
		if got != want {
			t.Errorf("Body = %q; want %q", got, want)
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		const errorMessage = "hello error"
		h := NewHandler(nil, func(ctx context.Context, r *http.Request) (*Response, error) {
			return nil, WithStatusCode(http.StatusUnprocessableEntity, errors.New(errorMessage))
		})
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		resp, err := srv.Client().Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if got, want := resp.StatusCode, http.StatusUnprocessableEntity; got != want {
			t.Errorf("StatusCode = %d; want %d", got, want)
		}
		got, err := readAllString(resp.Body)
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(got, errorMessage) {
			t.Errorf("Body = %q; want to contain %q", got, errorMessage)
		}
	})
}
