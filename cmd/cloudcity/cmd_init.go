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
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	slashpath "path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"zombiezen.com/go/bass/sigterm"
)

//go:embed template template/*.tmpl
var initTemplate embed.FS

type initCmd struct {
	dir        string
	modulePath string
	force      bool
}

func newInitCmd() *cobra.Command {
	cmd := new(initCmd)
	c := &cobra.Command{
		Use:   "init [options] [DIR]",
		Short: "Initialize a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cc *cobra.Command, args []string) error {
			if len(args) > 0 {
				cmd.dir = args[0]
			}
			return cmd.run(cc.Context())
		},
		DisableFlagsInUseLine: true,
	}
	c.Flags().StringVar(&cmd.modulePath, "module-path", "", "module path for go.mod")
	c.Flags().BoolVarP(&cmd.force, "force", "f", false, "force creating files, even if the directory is not empty")
	return c
}

func (cmd *initCmd) run(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("init: %w", err)
		}
	}()
	dir := cmd.dir
	if dir == "" {
		dir = "."
	}
	isEmpty, err := ensureDirectory(dir)
	if err != nil {
		return err
	}
	if !isEmpty && !cmd.force {
		fmt.Fprintln(os.Stderr, "cloudcity: directory is not empty; assuming project is already initialized.")
		fmt.Fprintln(os.Stderr, "Use --force if you want to overwrite files.")
		return nil
	}

	// Run `go mod init` before Go files are present.
	modInitCmd := exec.Command("go", "mod", "init")
	if cmd.modulePath != "" {
		modInitCmd.Args = append(modInitCmd.Args, cmd.modulePath)
	}
	modInitCmd.Dir = dir
	modInitCmd.Stdout = os.Stderr
	modInitCmd.Stderr = os.Stderr
	if err := sigterm.Run(ctx, modInitCmd); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	// Prepare template variables.
	var templateData struct {
		ProgramName string
		Author      string
		Year        int
		CSRFKey     string
	}
	templateData.Year = time.Now().Year()
	currentUser, err := user.Current()
	if err != nil {
		return err
	}
	templateData.Author = currentUser.Name
	modulePath, err := readModulePath(ctx, dir)
	if err != nil {
		return err
	}
	templateData.ProgramName = slashpath.Base(modulePath)
	var csrfKeyBits [32]byte
	if _, err := rand.Read(csrfKeyBits[:]); err != nil {
		return fmt.Errorf("generate CSRF key: %w", err)
	}
	templateData.CSRFKey = hex.EncodeToString(csrfKeyBits[:])
	funcs := template.FuncMap{
		"toEnv": func(s string) string {
			sb := new(strings.Builder)
			for _, c := range s {
				switch {
				case 'a' <= c && c <= 'z':
					sb.WriteRune(c - 'a' + 'A')
				case 'A' <= c && c <= 'Z' || '0' <= c && c <= '9':
					sb.WriteRune(c)
				default:
					sb.WriteRune('_')
				}
			}
			return sb.String()
		},
	}

	// Copy files into directory.
	const templateDir = "template"
	err = fs.WalkDir(initTemplate, templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := initTemplate.ReadFile(path)
		if err != nil {
			return err
		}
		const templateExt = ".tmpl"
		subdir, base := slashpath.Split(strings.TrimPrefix(path, templateDir+"/"))
		if strings.HasSuffix(path, templateExt) {
			tmpl, err := template.New(base).Funcs(funcs).Delims("/*{", "}*/").Parse(string(data))
			if err != nil {
				return err
			}
			buf := new(bytes.Buffer)
			if err := tmpl.Execute(buf, templateData); err != nil {
				return err
			}
			data = buf.Bytes()
			base = base[:len(base)-len(templateExt)]
		}
		dst := filepath.Join(dir, filepath.FromSlash(subdir), base)
		fmt.Fprintf(os.Stderr, "cloudcity: creating %s\n", dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o666); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Install Go dependencies.
	getCmd := exec.Command("go", "get",
		"github.com/gorilla/csrf@v1.7.0",
		"github.com/gorilla/handlers@v1.5.1",
		"github.com/gorilla/mux@v1.8.0",
		"github.com/yourbase/commons/ini@v0.9.1",
		"zombiezen.com/go/bass/sigterm@cb0af0b391a447f2a733aff1cf175e456c2d27af",
		"zombiezen.com/go/log@v1.0.3",
	)
	getCmd.Dir = dir
	getCmd.Stdout = os.Stderr
	getCmd.Stderr = os.Stderr
	if err := sigterm.Run(ctx, getCmd); err != nil {
		return err
	}
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyCmd.Stdout = os.Stderr
	tidyCmd.Stderr = os.Stderr
	if err := sigterm.Run(ctx, tidyCmd); err != nil {
		return err
	}

	// Install JavaScript dependencies and build.
	bcc := &buildClientCmd{
		install: true,
	}
	if err := bcc.build(ctx, dir); err != nil {
		return err
	}

	return nil
}

func readModulePath(ctx context.Context, dir string) (string, error) {
	listCmd := exec.Command("go", "list", "-m", "-json")
	listCmd.Dir = dir
	out := new(bytes.Buffer)
	listCmd.Stdout = out
	listCmd.Stderr = os.Stderr

	if err := sigterm.Run(ctx, listCmd); err != nil {
		return "", fmt.Errorf("read module path: go list: %w", err)
	}
	var module struct {
		Path string
	}
	if err := json.Unmarshal(out.Bytes(), &module); err != nil {
		return "", fmt.Errorf("read module path: parse go list output: %w", err)
	}
	return module.Path, nil
}

func ensureDirectory(path string) (isEmpty bool, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return false, err
	}
	if err := os.Mkdir(path, 0o777); err == nil {
		// If freshly created, no need to list directory contents.
		return true, nil
	} else if !os.IsExist(err) {
		return false, err
	}
	dirContents, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, ent := range dirContents {
		if !strings.HasPrefix(ent.Name(), ".") {
			return false, nil
		}
	}
	return true, nil
}
