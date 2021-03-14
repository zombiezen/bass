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
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestInitCmd(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	cmd := &initCmd{
		dir:        dir,
		modulePath: "example.com/foo",
	}
	if err := cmd.run(ctx); err != nil {
		t.Error("From run:", err)
	}

	buildCmd := exec.Command("go", "build")
	buildCmd.Dir = dir
	output := new(strings.Builder)
	buildCmd.Stdout = output
	buildCmd.Stderr = output
	err := buildCmd.Run()
	t.Logf("go build output:\n%s", output.String())
	if err != nil {
		t.Error("go build:", err)
	}
}
