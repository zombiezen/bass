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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"zombiezen.com/go/bass/sigterm"
)

type buildClientCmd struct {
	compile bool
	install bool
}

func newBuildClientCmd() *cobra.Command {
	cmd := new(buildClientCmd)
	c := &cobra.Command{
		Use:   "build",
		Short: "Bundle the client-side code",
		Args:  cobra.NoArgs,
		RunE: func(cc *cobra.Command, args []string) error {
			return cmd.run(cc.Context())
		},
	}
	c.Flags().BoolVarP(&cmd.compile, "compile", "c", true, "perform type-checking")
	c.Flags().BoolVarP(&cmd.install, "install", "i", false, "install dependencies")
	return c
}

func (cmd *buildClientCmd) run(ctx context.Context) (err error) {
	root, err := findGoModuleDir(ctx, ".")
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}
	return cmd.build(ctx, root)
}

func (cmd *buildClientCmd) build(ctx context.Context, root string) error {
	clientDir := filepath.Join(root, clientDirectoryName)

	if cmd.install {
		fmt.Fprintln(os.Stderr, "## npm install ##")
		npmInstallCmd := exec.Command("npm", "install")
		npmInstallCmd.Dir = clientDir
		npmInstallCmd.Stdout = os.Stderr
		npmInstallCmd.Stderr = os.Stderr
		if err := sigterm.Run(ctx, npmInstallCmd); err != nil {
			return fmt.Errorf("build client: npm install: %w", err)
		}
	}

	if cmd.compile {
		fmt.Fprintln(os.Stderr, "## npm run-script compile --if-present ##")
		npmCompileCmd := exec.Command("npm", "run-script", "compile", "--if-present")
		npmCompileCmd.Dir = clientDir
		npmCompileCmd.Stdout = os.Stderr
		npmCompileCmd.Stderr = os.Stderr
		if err := sigterm.Run(ctx, npmCompileCmd); err != nil {
			return fmt.Errorf("build client: npm run compile: %w", err)
		}
	}

	fmt.Fprintln(os.Stderr, "## npm run-script build ##")
	npmBuildCmd := exec.Command("npm", "run-script", "build")
	npmBuildCmd.Dir = clientDir
	npmBuildCmd.Stdout = os.Stderr
	npmBuildCmd.Stderr = os.Stderr
	if err := sigterm.Run(ctx, npmBuildCmd); err != nil {
		return fmt.Errorf("build client: npm run build: %w", err)
	}

	return nil
}
