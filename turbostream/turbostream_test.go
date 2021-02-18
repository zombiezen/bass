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

package turbostream

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

func TestMarshalText(t *testing.T) {
	tests := []struct {
		name     string
		action   *Action
		wantHTML string
	}{
		{
			name:     "Nil",
			action:   nil,
			wantHTML: "",
		},
		{
			name: "Append",
			action: &Action{
				Type:     Append,
				TargetID: "messages",
				Template: staticTemplate(
					`<div id="message_1">` +
						`This div will be appended to the element with the DOM ID "messages".` +
						`</div>`),
			},
			wantHTML: `<turbo-stream action="append" target="messages">` +
				`<template>` +
				`<div id="message_1">` +
				`This div will be appended to the element with the DOM ID "messages".` +
				`</div>` +
				`</template>` +
				`</turbo-stream>`,
		},
		{
			name: "Remove",
			action: &Action{
				Type:     Remove,
				TargetID: "message_1",
			},
			wantHTML: `<turbo-stream action="remove" target="message_1"></turbo-stream>`,
		},
		{
			name: "SpecialIDChars",
			action: &Action{
				Type:     Remove,
				TargetID: "message&1",
			},
			wantHTML: `<turbo-stream action="remove" target="message&amp;1"></turbo-stream>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotHTML, err := test.action.MarshalText()
			if err != nil {
				t.Fatal("MarshalText:", err)
			}
			got, err := htmlTokens(bytes.NewReader(gotHTML))
			if err != nil {
				t.Fatalf("parse HTML: %v\ngot:\n%s", gotHTML, err)
			}
			want, err := htmlTokens(strings.NewReader(test.wantHTML))
			if err != nil {
				t.Fatalf("could not parse wanted HTML: %v", err)
			}
			if !cmp.Equal(want, got) {
				t.Errorf("HTML did not match\ngot:\n%s\nwant:\n%s", gotHTML, test.wantHTML)
			}
		})
	}
}

type staticTemplate string

func (s staticTemplate) Execute(w io.Writer, data interface{}) error {
	_, err := io.WriteString(w, string(s))
	return err
}

func htmlTokens(r io.Reader) ([]html.Token, error) {
	t := html.NewTokenizer(r)
	var tokens []html.Token
	for t.Next() != html.ErrorToken {
		tok := t.Token()
		if tok.Type == html.TextToken {
			tok.Data = strings.TrimSpace(tok.Data)
			if tok.Data == "" {
				continue
			}
		}
		tokens = append(tokens, tok)
	}
	if err := t.Err(); err != io.EOF {
		return tokens, err
	}
	return tokens, nil
}
