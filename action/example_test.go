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

package action_test

import (
	"context"
	"net/http"
	"testing/fstest"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"zombiezen.com/go/bass/action"
)

// base.html file
const baseHTML = `<!DOCTYPE html>
<html>
	<title>
		{{- block "title" . }}{{ end -}}
	</title>
	<body>
		{{- block "body" . }}{{ end -}}
	</body>
</html>`

// index.html file
const indexHTML = `<!DOCTYPE html>
{{ define "title" -}}
	Home
{{- end }}

{{ define "body" -}}
	<h1>Hello, {{ .Subject }}!</h1>
{{- end }}`

func Example() {
	// Typically, you would load these from disk
	// either by using embed or with os.DirFS.
	templateFiles := fstest.MapFS{
		"base.html":  {Data: []byte(baseHTML)},
		"index.html": {Data: []byte(indexHTML)},
	}

	// NewHandler can be used much like http.HandlerFunc,
	// but it also takes in templates.
	// For more advanced features, see action.Config.
	indexHandler := action.NewHandler(templateFiles, index)

	// Add the handler to your router of choice:
	router := mux.NewRouter()
	router.Handle("/", handlers.MethodHandler{
		http.MethodGet:  indexHandler,
		http.MethodHead: indexHandler,
	})
	http.ListenAndServe(":8080", router)
}

// index is an action.Func.
func index(ctx context.Context, r *http.Request) (*action.Response, error) {
	// Responses
	return &action.Response{
		HTMLTemplate: "index.html",
		TemplateData: map[string]any{
			"Subject": "World",
		},
	}, nil
}
