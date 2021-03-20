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

package main

import (
	"go/parser"
	"testing"
)

func TestFormatExpr(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{
			expr: "x",
			want: "x",
		},
		{
			expr: `"foo"`,
			want: `"foo"`,
		},
		{
			expr: "2+x",
			want: "2 + x",
		},
		{
			expr: "f(x, (y + 1) * 2)",
			want: "f(x, (y + 1) * 2)",
		},
		{
			expr: "f(*foo)",
			want: "f(*foo)",
		},
		{
			expr: "f(-x)",
			want: "f(-x)",
		},
		{
			expr: "arr[:]",
			want: "arr[:]",
		},
		{
			expr: "arr[n:]",
			want: "arr[n:]",
		},
		{
			expr: "arr[:n]",
			want: "arr[:n]",
		},
		{
			expr: "arr[:n:n]",
			want: "arr[:n:n]",
		},
	}
	for _, test := range tests {
		expr, err := parser.ParseExpr(test.expr)
		if err != nil {
			t.Error(err)
			continue
		}
		if got := formatExpr(expr); got != test.want {
			t.Errorf("formatExpr(parser.ParseExpr(%q)) = %q; want %q", test.expr, got, test.want)
		}
	}
}
