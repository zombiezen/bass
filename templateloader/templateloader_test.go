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

package templateloader

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAddPartials(t *testing.T) {
	dir := filepath.Join("testdata", "AddPartials")
	tmpl, err := template.ParseFiles(filepath.Join(dir, "test.html"))
	if err != nil {
		t.Fatal(err)
	}

	gotTemplate, err := AddPartials(tmpl, os.DirFS(dir))
	if err != nil {
		t.Fatal("AddPartials:", err)
	}
	if gotTemplate != tmpl {
		t.Errorf("AddPartials(%p, os.DirFS(%q)) = %p; want %p", tmpl, dir, gotTemplate, tmpl)
	}
	var templateNames []string
	for _, sub := range gotTemplate.Templates() {
		templateNames = append(templateNames, sub.Name())
	}
	t.Logf("Defined templates: %s", strings.Join(templateNames, ", "))

	got := new(strings.Builder)
	if err := tmpl.Execute(got, nil); err != nil {
		t.Fatal(err)
	}
	const want = "<h1>Hello, World!</h1>\n" +
		"<p>Top-level partial.</p>"
	if diff := cmp.Diff(got.String(), want); diff != "" {
		t.Errorf("template output (-want +got):\n%s", diff)
	}
}
