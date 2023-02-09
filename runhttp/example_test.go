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

package runhttp_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	"zombiezen.com/go/bass/runhttp"
)

func ExampleServe() {
	// Set up the context to be canceled when receiving SIGINT.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Set up http.Server.
	portString := os.Getenv("PORT")
	if portString == "" {
		portString = "8080"
	}
	srv := &http.Server{
		Addr: ":" + portString,
	}

	// Run the http.Server.
	err := runhttp.Serve(ctx, srv, &runhttp.Options{
		OnStartup: func(ctx context.Context, addr net.Addr) {
			s := addr.String()
			if tcpAddr, ok := addr.(*net.TCPAddr); ok {
				s = fmt.Sprintf("localhost:%d", tcpAddr.Port)
			}
			log.Printf("Listening on http://%s", s)
		},
		OnShutdown: func(ctx context.Context) {
			log.Printf("Shutting down...")
		},
		OnShutdownError: func(ctx context.Context, err error) {
			log.Printf("During shutdown: %v", err)
		},
	})
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
