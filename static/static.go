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

// Package static provides a static file HTTP handler that sends content-based
// cache headers.
package static

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	slashpath "path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Handler is an HTTP handler for a file system.
type Handler struct {
	fs      fs.FS
	errFunc func(ctx context.Context, path string, err error) string
}

// NewHandler returns a new Handler that serves the given file system.
func NewHandler(fsys fs.FS) *Handler {
	return &Handler{
		fs:      fsys,
		errFunc: defaultErrorFunc,
	}
}

// ServeHTTP serves the file named by the request's path from the Handler's
// file system.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(slashpath.Clean(r.URL.Path), "/")
	if path == "" {
		path = "."
	}
	h.ServeFile(w, r, path)
}

// ServeFile serves the given file from the Handler's file system. It primarily
// uses net/http.ServeContent, but sets a content-based ETag first.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if !fs.ValidPath(path) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(w, "Only GET and HEAD allowed on resource", http.StatusMethodNotAllowed)
		return
	}
	f, err := h.fs.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.error(ctx, w, path, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		h.error(ctx, w, path, err)
		return
	}
	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			// Redirect if URL does not end in slash.
			localRedirect(w, r, slashpath.Base(r.URL.Path)+"/")
			return
		}
		contents, err := f.(fs.ReadDirFile).ReadDir(-1)
		if err != nil {
			h.error(ctx, w, path, err)
			return
		}
		dirList(w, contents)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") {
		// Redirect if non-directory URL ends in slash.
		localRedirect(w, r, "../"+slashpath.Base(r.URL.Path))
		return
	}
	s, err := toSeeker(f, info.Size())
	if err != nil {
		h.error(ctx, w, path, err)
		return
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, s); err != nil {
		h.error(ctx, w, path, err)
		return
	}
	if _, err := s.Seek(0, io.SeekStart); err != nil {
		h.error(ctx, w, path, err)
		return
	}
	w.Header().Set("ETag", `"`+hex.EncodeToString(hash.Sum(nil))+`"`)
	http.ServeContent(w, r, path, time.Time{}, s)
}

// SetErrorFunc sets the error callback for the Handler. The function is
// responsible for logging the error and returns the error string that should
// be sent back in response. The default error callback returns the
// error's message.
//
// SetErrorFunc must not be called concurrently with ServeHTTP.
func (h *Handler) SetErrorFunc(f func(ctx context.Context, path string, err error) string) {
	if f == nil {
		panic("nil function passed to static.Handler.SetErrorFunc")
	}
	h.errFunc = f
}

func (h *Handler) error(ctx context.Context, w http.ResponseWriter, path string, err error) {
	msg := h.errFunc(ctx, path, err)
	http.Error(w, msg, http.StatusInternalServerError)
}

func defaultErrorFunc(ctx context.Context, path string, err error) string {
	return err.Error()
}

func dirList(w http.ResponseWriter, contents []fs.DirEntry) {
	contents = append([]fs.DirEntry(nil), contents...)
	sort.Slice(contents, func(i, j int) bool {
		return contents[i].Name() < contents[j].Name()
	})
	buf := new(bytes.Buffer)
	buf.WriteString("<!DOCTYPE html>\n<html>\n<body>\n<ul>\n")
	for _, ent := range contents {
		buf.WriteString(`<li><a href="`)
		name := ent.Name()
		buf.WriteString((&url.URL{Path: name}).String())
		if ent.IsDir() {
			buf.WriteString("/")
		}
		buf.WriteString(`">`)
		buf.WriteString(html.EscapeString(name))
		buf.WriteString("</a></li>\n")
	}
	buf.WriteString("</ul>\n</body>\n</html>\n")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

// toSeeker attempts to return a seekable view into r, either by detecting a
// Seek on r or by reading its contents into memory. toSeeker may consume r:
// future reads should be made to the returned io.ReadSeeker, not to r.
func toSeeker(r io.Reader, size int64) (io.ReadSeeker, error) {
	const maxMemory = 4 << 20 // 4 MiB
	if rs, ok := r.(io.ReadSeeker); ok {
		return rs, nil
	}
	if size > maxMemory {
		return nil, fmt.Errorf("read file into memory: too large (%d bytes)", size)
	}
	data, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return nil, fmt.Errorf("read file into memory: %w", err)
	}
	return bytes.NewReader(data), nil
}
