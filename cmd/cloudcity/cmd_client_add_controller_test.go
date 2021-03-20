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

import "testing"

func TestControllerNameToPath(t *testing.T) {
	tests := []struct {
		name      string
		want      string
		wantError bool
	}{
		{
			name: "clipboard",
			want: "clipboard_controller.ts",
		},
		{
			name: "date-picker",
			want: "date_picker_controller.ts",
		},
		{
			name: "users--list-item",
			want: "users/list_item_controller.ts",
		},
		{
			name:      "users---list-item",
			wantError: true,
		},
		{
			name:      "date_picker",
			wantError: true,
		},
		{
			name:      "date picker",
			wantError: true,
		},
		{
			name:      "clipboard-",
			wantError: true,
		},
		{
			name:      "-clipboard",
			wantError: true,
		},
		{
			name:      "",
			wantError: true,
		},
	}
	for _, test := range tests {
		got, err := controllerNameToPath(test.name)
		if err != nil {
			if test.wantError {
				t.Logf("controllerNameToPath(%q) = _, %v", test.name, err)
			} else {
				t.Errorf("controllerNameToPath(%q) = _, %v; want %q, <nil>", test.name, err, test.want)
			}
			continue
		}
		if test.wantError {
			t.Errorf("controllerNameToPath(%q) = %q, <nil>; want _, <error>", test.name, got)
			continue
		}
		if got != test.want {
			t.Errorf("controllerNameToPath(%q) = %q, <nil>; want %q, <nil>", test.name, got, test.want)
		}
	}
}
