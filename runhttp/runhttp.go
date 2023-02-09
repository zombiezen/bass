// Copyright 2023 The Bass Authors
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

// Package runhttp provides functions for running an HTTP server.
package runhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// Options holds the optional arguments to [Serve].
type Options struct {
	// Listener will be used if non-nil to serve on.
	// Otherwise, the [*http.Server.Addr] will be used to listen for TCP connections.
	Listener net.Listener
	// OnStartup will be called after the listener is ready,
	// but before serving starts.
	OnStartup func(context.Context, net.Addr)
	// OnShutdown will be called after the Context is Done,
	// but before [*http.Server.Shutdown] starts.
	OnShutdown func(context.Context)
	// OnShutdownError will be called if [*http.Server.Shutdown] returns a non-nil error.
	OnShutdownError func(context.Context, error)
}

// Serve runs the given HTTP server until the context is Done.
func Serve(ctx context.Context, srv *http.Server, opts *Options) error {
	if srv.BaseContext == nil {
		srv2 := new(http.Server)
		*srv2 = *srv
		srv2.BaseContext = func(net.Listener) context.Context { return ctx }
	}

	var l net.Listener
	if opts != nil {
		l = opts.Listener
	}
	if l == nil {
		addr := srv.Addr
		if addr == "" {
			addr = ":http"
		}
		var err error
		l, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		// [*http.Server.Serve] will close l.
	}

	serveFinished := make(chan struct{})
	idleConnsClosed := make(chan struct{})
	go func() {
		defer close(idleConnsClosed)
		select {
		case <-ctx.Done():
			if opts != nil && opts.OnShutdown != nil {
				opts.OnShutdown(ctx)
			}
			err := srv.Shutdown(context.Background())
			if err != nil && opts != nil && opts.OnShutdownError != nil {
				opts.OnShutdownError(ctx, err)
			}
		case <-serveFinished:
		}
	}()

	if opts != nil && opts.OnStartup != nil {
		opts.OnStartup(ctx, l.Addr())
	}
	err := srv.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	close(serveFinished)
	<-idleConnsClosed
	return err
}
