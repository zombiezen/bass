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

// cloudcity is a CLI for bootstrapping Go projects.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"zombiezen.com/go/bass/sigterm"
)

const clientDirectoryName = "client"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), sigterm.Signals()...)
	rootCmd := &cobra.Command{
		Use:           "cloudcity",
		Short:         "CLI for bootstrapping Go projects",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.AddCommand(
		newInitCmd(),
		newServerCmd(),
	)

	clientCmd := &cobra.Command{
		Use:           "client",
		Short:         "Mange client-side code",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	clientCmd.AddCommand(
		newAddControllerCmd(),
		newBuildClientCmd(),
	)
	rootCmd.AddCommand(clientCmd)

	err := rootCmd.ExecuteContext(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cloudcity:", err)
		os.Exit(1)
	}
}

// findGoModuleDir locates the root Go module directory.
func findGoModuleDir(ctx context.Context, dir string) (string, error) {
	c := exec.Command("go", "env", "GOMOD")
	stdout := new(strings.Builder)
	c.Stdout = stdout
	stderr := new(strings.Builder)
	c.Stderr = stderr
	c.Dir = dir
	if err := sigterm.Run(ctx, c); err != nil {
		if stderr.Len() == 0 {
			return "", fmt.Errorf("find go module directory for %s: %w", dir, err)
		}
		return "", fmt.Errorf("find go module directory for %s: %w; stderr:\n%s", dir, err, strings.TrimSuffix(stderr.String(), "\n"))
	}
	gomod := strings.TrimSuffix(stdout.String(), "\n")
	if gomod == "" || gomod == "/dev/null" || gomod == "NUL" {
		return "", fmt.Errorf("find go module directory for %s: not found", gomod)
	}
	return filepath.Dir(gomod), nil
}
