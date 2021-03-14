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

package static_test

import (
	"net/http"
	"os"

	"zombiezen.com/go/bass/static"
)

func Example() {
	// Create a static handler with the desired file system.
	// In this case, we use a directory on the local filesystem.
	staticHandler := static.NewHandler(os.DirFS("path/to/dir"))

	// Serve the handler under the prefix "/dir/".
	// We use the http.StripPrefix function to avoid "/dir/foo.txt" requesting
	// "path/to/dir/dir/foo.txt".
	http.Handle("/dir/", http.StripPrefix("/dir", staticHandler))
}
