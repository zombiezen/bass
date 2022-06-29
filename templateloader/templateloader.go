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

// Package templateloader provides functions to parse templates in a manner
// similar to Django and Rails with a base template and partial templates.
package templateloader

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	slashpath "path"
	"strings"
	texttemplate "text/template"
)

// Base parses base.html and any partial templates present in the file system.
func Base(fsys fs.FS, funcs template.FuncMap) (*template.Template, error) {
	const name = "base.html"
	tmpl, err := parse(template.New(name).Funcs(funcs), fsys, name)
	if err != nil {
		return nil, err
	}
	return AddPartials(tmpl, fsys)
}

// AddPartials searches the given file system for partial templates,
// parses them, and adds them to t. Partial templates are files that start with
// an underscore ("_") and end with the extension ".html". The underscore and
// ".html" are stripped from the template name, so "shared/_menu.html" will be
// available as "shared/menu".
func AddPartials(t *template.Template, fsys fs.FS) (*template.Template, error) {
	return addPartials(t, fsys, ".html")
}

// AddTextPartials searches the given file system for partial templates,
// parses them, and adds them to t. Partial templates are files that start with
// an underscore ("_") and end with the extension ".txt". The underscore and
// ".txt" are stripped from the template name, so "shared/_menu.txt" will be
// available as "shared/menu".
func AddTextPartials(t *texttemplate.Template, fsys fs.FS) (*texttemplate.Template, error) {
	return addPartials(t, fsys, ".txt")
}

func addPartials[T templateType[T]](t T, fsys fs.FS, ext string) (T, error) {
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		dir, name := slashpath.Split(strings.TrimPrefix(path, "./"))
		if d.IsDir() {
			// Descend into any visible directories.
			if strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !(strings.HasPrefix(name, "_") && strings.HasSuffix(name, ext)) {
			// Not a partial template: ignore.
			return nil
		}
		templateName := dir + name[1:len(name)-len(ext)]
		_, err = parse(t.New(templateName), fsys, path)
		return err
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return t, nil
}

// Extend returns a duplicate of a base template, including all associated
// templates, that also includes templates parsed from the given file in the
// file system. It returns an error if the base template has already been
// executed.
func Extend(base *template.Template, fsys fs.FS, name string) (*template.Template, error) {
	return extend(base, fsys, name)
}

func extend[T templateType[T]](base T, fsys fs.FS, name string) (T, error) {
	var zero T
	tmpl, err := base.Clone()
	if err != nil {
		return zero, err
	}
	if _, err := parse(tmpl.New(name), fsys, name); err != nil {
		return zero, err
	}
	return tmpl, nil
}

// ParseFile parses a single file (not a glob pattern) as a template body for t.
func ParseFile(t *template.Template, fsys fs.FS, filename string) (*template.Template, error) {
	return parse(t, fsys, filename)
}

// ParseTextFile parses a single file (not a glob pattern) as a template body for t.
func ParseTextFile(t *texttemplate.Template, fsys fs.FS, filename string) (*texttemplate.Template, error) {
	return parse(t, fsys, filename)
}

func parse[T templateType[T]](t T, fsys fs.FS, filename string) (T, error) {
	text, err := readString(fsys, filename)
	if err != nil {
		var zero T
		return zero, err
	}
	return t.Parse(text)
}

func readString(fsys fs.FS, filename string) (string, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return "", err
	}
	content := new(strings.Builder)
	_, err = io.Copy(content, f)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}
	return content.String(), nil
}

type templateType[T any] interface {
	New(name string) T
	Clone() (T, error)
	Parse(text string) (T, error)
}
