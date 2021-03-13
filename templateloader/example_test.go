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

package templateloader_test

import (
	"fmt"
	"os"
	"testing/fstest"

	"zombiezen.com/go/bass/templateloader"
)

func Example() {
	// For demonstration purposes, all of the templates are defined here.
	// In a real program, you would place files into a directory and either
	// use go:embed or os.DirFS.
	fsys := fstest.MapFS{
		// Base template is always defined in base.html.
		"base.html": {
			Data: []byte(
				`<html>{{ template "head" . }}` + "\n" +
					`<body>{{ block "content" . }}{{ end }}</body></html>`),
		},

		// Regular templates fill in blocks from base.html.
		"hello.html": {
			Data: []byte(`{{ define "content" }}Hello, {{ . }}!{{ end }}`),
		},

		// Partial template filenames start with an underscore. The underscore and
		// the extension are stripped for the template name.
		"_head.html": {
			Data: []byte(`<head><title>Template Test</title></head>`),
		},
	}

	// Load the base template and partials first.
	base, err := templateloader.Base(fsys, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}

	// Obtain a specific page template with Extend.
	tmpl, err := templateloader.Extend(base, fsys, "hello.html")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}
	if err := tmpl.Execute(os.Stdout, "World"); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}

	// Output:
	// <html><head><title>Template Test</title></head>
	// <body>Hello, World!</body></html>
}
