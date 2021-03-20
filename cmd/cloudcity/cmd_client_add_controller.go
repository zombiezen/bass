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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type addControllerCmd struct {
	name string
}

func newAddControllerCmd() *cobra.Command {
	cmd := new(addControllerCmd)
	c := &cobra.Command{
		Use:   "add-controller [options] NAME",
		Short: "Create a Stimulus controller",
		Args:  cobra.ExactArgs(1),
		RunE: func(cc *cobra.Command, args []string) error {
			cmd.name = args[0]
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
	}
	return c
}

//go:embed controller.ts
var controllerTemplate []byte

func (cmd *addControllerCmd) run(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("add controller: %w", err)
		}
	}()
	dir, err := findGoModuleDir(ctx, ".")
	if err != nil {
		return err
	}
	controllerPath, err := controllerNameToPath(cmd.name)
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, clientDirectoryName, "controllers", filepath.FromSlash(controllerPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	if err := createFile(dst, controllerTemplate); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Created %s\n", dst)
	fmt.Fprintf(os.Stderr, "You can add the controller to your HTML with data-controller=\"%s\"\n", cmd.name)
	return nil
}

func controllerNameToPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("invalid controller name %q: empty", name)
	}
	const ext = "_controller.ts"
	pathBuilder := new(strings.Builder)
	pathBuilder.Grow(len(name) + len(ext))
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			pathBuilder.WriteByte(c)
		case c == '-':
			if i == 0 {
				return "", fmt.Errorf("invalid controller name %q: leading dash not allowed", name)
			}
			if i+1 >= len(name) {
				return "", fmt.Errorf("invalid controller name %q: trailing dash not allowed", name)
			}
			if name[i+1] == '-' {
				// Double dash.
				if i+2 >= len(name) {
					return "", fmt.Errorf("invalid controller name %q: trailing dash not allowed", name)
				}
				if name[i+2] == '-' {
					return "", fmt.Errorf("invalid controller name %q: triple dash not allowed", name)
				}
				pathBuilder.WriteByte('/')
				i++ // skip second dash
				continue
			}
			pathBuilder.WriteByte('_')
		default:
			return "", fmt.Errorf("invalid controller name %q: unsupported character %q", name, c)
		}
	}
	pathBuilder.WriteString(ext)
	return pathBuilder.String(), nil
}

func createFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write %s: %w", path, closeErr)
	}
	return nil
}
