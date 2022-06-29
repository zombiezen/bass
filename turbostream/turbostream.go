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

// Package turbostream provides framing for fragments of HTML to be used with
// the Turbo Streams client-side library. Read more at https://turbo.hotwire.dev/handbook/streams
package turbostream

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"

	"zombiezen.com/go/bass/accept"
)

// ContentType is the MIME type of a Turbo Stream response.
const ContentType = "text/vnd.turbo-stream.html"

// IsSupported reports whether the request header indicate that a Turbo Stream
// response is supported.
func IsSupported(reqHeader http.Header) bool {
	h, err := accept.ParseHeader(reqHeader.Get("Accept"))
	if err != nil {
		return false
	}
	return h.Quality(ContentType, map[string]string{"charset": "utf-8"}) > 0
}

// Render sends Turbo Stream actions in response to a form submission.
// See https://turbo.hotwire.dev/handbook/streams#streaming-from-http-responses
// for an overview.
//
// Render does not write any data or set headers if it returns an error.
func Render(w http.ResponseWriter, actions ...*Action) error {
	buf := new(bytes.Buffer)
	for _, a := range actions {
		if err := a.appendTo(buf); err != nil {
			return err
		}
		if a != nil {
			buf.WriteByte('\n')
		}
	}
	w.Header().Set("Content-Type", ContentType+"; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	io.Copy(w, buf) // ignore errors, since we already wrote
	return nil
}

// ActionType is the value of the turbo-stream element's action attribute.
type ActionType string

// Action types defined in https://turbo.hotwire.dev/reference/streams
const (
	// Append appends the content to the container designated by the target DOM ID.
	Append ActionType = "append"
	// Prepend prepends the content to the container designated by the target DOM ID.
	Prepend ActionType = "prepend"
	// Replace replaces designated by the target DOM ID with the action's content.
	Replace ActionType = "replace"
	// Update replaces the content in the container designated by the target
	// DOM ID with the action's content.
	Update ActionType = "update"
	// Remove removes the element designated by the target DOM ID. The action's
	// Template must be nil.
	Remove ActionType = "remove"
)

// IsValid reports whether t is one of the defined action types.
func (t ActionType) IsValid() bool {
	return t == Append || t == Prepend || t == Replace || t == Update || t == Remove
}

// Action is a single instruction on how to modify an HTML document.
type Action struct {
	Type     ActionType
	TargetID string
	Template Executer
	Data     interface{}
}

// Executer is the interface that wraps the Execute method of templates.
// Execute applies a parsed template to the specified data object, writing
// the output to wr.
type Executer interface {
	Execute(wr io.Writer, data interface{}) error
}

// NewRemove returns a new action with type Remove.
func NewRemove(id string) *Action {
	return &Action{Type: Remove, TargetID: id}
}

// MarshalText renders the template as HTML. If the Action is nil, then it
// returns (nil, nil).
func (a *Action) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	if err := a.appendTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *Action) validate() error {
	if a == nil {
		return nil
	}
	if !a.Type.IsValid() {
		return fmt.Errorf("invalid action %q", a.Type)
	}
	if a.TargetID == "" {
		return fmt.Errorf("target empty")
	}
	if a.Type == Remove && (a.Template != nil || a.Data != nil) {
		return fmt.Errorf("%s %s: content not empty", a.Type, a.TargetID)
	}
	return nil
}

func (a *Action) appendTo(buf *bytes.Buffer) error {
	if a == nil {
		return nil
	}
	if err := a.validate(); err != nil {
		return fmt.Errorf("marshal turbo-stream: %w", err)
	}
	buf.WriteString(`<turbo-stream action="`)
	buf.WriteString(string(a.Type))
	buf.WriteString(`" target="`)
	buf.WriteString(html.EscapeString(a.TargetID))
	buf.WriteString(`">`)
	if a.Type != Remove {
		buf.WriteString("\n\t<template>")
		if a.Template != nil {
			if err := a.Template.Execute(buf, a.Data); err != nil {
				return fmt.Errorf("marshal turbo-stream: %s %s: %w", a.Type, a.TargetID, err)
			}
		}
		buf.WriteString("</template>\n")
	}
	buf.WriteString("</turbo-stream>")
	return nil
}
