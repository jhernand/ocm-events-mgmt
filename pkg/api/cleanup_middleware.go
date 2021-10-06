/*
Copyright (c) 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains an HTTP middleware that performs some cleanups in the requests received for
// the API, like removing trailing slashes.

package api

import (
	"net/http"
	"strings"
)

// CleanupMiddleware creates a new HTTP middleware that performs cleanups in the requests received
// for the API, like removing trailing slashes.
func CleanupMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for len(path) > 1 && strings.HasSuffix(path, "/") {
			path = path[0 : len(path)-1]
		}
		r.URL.Path = path
		next.ServeHTTP(w, r)
	})
}
