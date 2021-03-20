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
	slashpath "path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"zombiezen.com/go/bass/sigterm"
)

type serverCmd struct {
	args          []string
	serverPackage string
	port          int
}

func newServerCmd() *cobra.Command {
	cmd := new(serverCmd)
	c := &cobra.Command{
		Use:     "server [options] [ -- ARG [...] ]",
		Short:   "Run a development server",
		Aliases: []string{"serve"},
		RunE: func(cc *cobra.Command, args []string) error {
			cmd.args = args
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
	}
	c.Flags().StringVar(&cmd.serverPackage, "package", ".", "Import path of Go server to run")
	c.Flags().IntVarP(&cmd.port, "port", "p", 8080, "Port to listen on")
	return c
}

func (cmd *serverCmd) run(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("server: %w", err)
		}
	}()
	root, err := findGoModuleDir(ctx, ".")
	if err != nil {
		return err
	}
	pkgs, err := listPackages(ctx, cmd.serverPackage)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("%s did not match any packages", cmd.serverPackage)
	}
	if len(pkgs) > 1 {
		return fmt.Errorf("%s refers to multiple packages", cmd.serverPackage)
	}
	serverPackage := pkgs[0]

	if err := (&buildClientCmd{compile: true}).build(ctx, root); err != nil {
		return err
	}

	relProgramPath := slashpath.Base(serverPackage)
	if runtime.GOOS == "windows" {
		relProgramPath += ".exe"
	}
	buildCmd := exec.Command("go", "build", "-o="+relProgramPath, "--", serverPackage)
	buildCmd.Stdout = os.Stderr
	buildCmd.Stderr = os.Stderr
	fmt.Fprintf(os.Stderr, "## go build -o %s %s ##\n", relProgramPath, serverPackage)
	if err := sigterm.Run(ctx, buildCmd); err != nil {
		return err
	}

	absProgramPath, err := filepath.Abs(relProgramPath)
	if err != nil {
		return err
	}
	// TODO(soon): "-client=" + filepath.Join(root, clientDirectoryName)
	serverCmd := exec.Command(absProgramPath, cmd.args...)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	serverCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", cmd.port))
	fmt.Fprintf(os.Stderr, "## %s ##\n", strings.Join(serverCmd.Args, " "))
	return sigterm.Run(ctx, serverCmd)
}

func listPackages(ctx context.Context, pattern string) ([]string, error) {
	c := exec.Command("go", "list", "--", pattern)
	stdout := new(strings.Builder)
	c.Stdout = stdout
	c.Stderr = os.Stderr
	if err := sigterm.Run(ctx, c); err != nil {
		return nil, fmt.Errorf("list go packages: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	return lines, nil
}
