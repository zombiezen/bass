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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zombiezen.com/go/bass/accept"
)

func TestDefaultTransformError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "Generic",
			err:            errors.New("bork"),
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       "bork\n",
		},
		{
			name:           "ErrNotFound",
			err:            ErrNotFound,
			wantStatusCode: http.StatusNotFound,
			wantBody:       "404 not found\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			resp := defaultTransformError(test.err)
			rec := httptest.NewRecorder()
			resp.render(ctx, rec, &renderOptions{
				reqMethod: http.MethodGet,
				reqPath:   "/foo",
				acceptHeader: accept.Header{
					{Range: "*/*", Quality: 1.0},
				},
			})

			got := rec.Result()
			defer got.Body.Close()
			if got.StatusCode != test.wantStatusCode {
				t.Errorf("StatusCode = %d (%s); want %d (%s)",
					got.StatusCode, http.StatusText(got.StatusCode),
					test.wantStatusCode, http.StatusText(test.wantStatusCode))
			}
			if got, want := got.Header.Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
				t.Errorf("Content-Type = %q; want %q", got, want)
			}
			gotBody, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(gotBody), test.err.Error(); !strings.Contains(got, want) {
				t.Errorf("body = %q; want to contain %q", got, want)
			}
		})
	}
}
