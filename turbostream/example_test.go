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

package turbostream_test

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"zombiezen.com/go/bass/turbostream"
)

func ExampleRender() {
	// In this example, we write an HTTP response to a response recorder.
	// In a real program, this would be the first argument to an http.Handler.
	w := httptest.NewRecorder()

	// Use html/template to build HTML.
	tmpl := template.Must(template.New("message.html").Parse(
		`<div id="message_{{ .ID }}">` + "\n" +
			"This div will be appended to the element with the DOM ID <q>messages</q>.\n" +
			`</div>`))

	// Call turbostream.Render to send the page modifications.
	err := turbostream.Render(w, &turbostream.Action{
		Type:     turbostream.Append,
		TargetID: "messages",
		Template: tmpl.Lookup("message.html"),
		Data:     struct{ ID int64 }{ID: 1},
	})
	if err != nil {
		// Render does nothing if the actions fail to render.
		// In this example, we serve an HTTP 500.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For demonstration, print out the response body to stdout.
	response := w.Result()
	io.Copy(os.Stdout, response.Body)

	// Output:
	// <turbo-stream action="append" target="messages">
	// 	<template><div id="message_1">
	// This div will be appended to the element with the DOM ID <q>messages</q>.
	// </div></template>
	// </turbo-stream>
}
