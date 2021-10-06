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

// This file contains the implementations of the HTTP handler wrapper that creates HTTP handlers
// that writse the details of the requests and responses to the log.

package logging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sdklogging "github.com/openshift-online/ocm-sdk-go/logging"
)

// HandlerWrapperBuilder contains the data and logic needed to create logging handler wrappers.
type HandlerWrapperBuilder struct {
	logger sdklogging.Logger
}

// HandlerWrapper wraps HTTP handlers and writes to the log some details of the request and
// responses.
type HandlerWrapper struct {
	logger sdklogging.Logger
}

// handler is the actual implementation of the http.Handler interface that writes to the log the
// details of the request and responses and calls the wrapped handler.
type handler struct {
	logger sdklogging.Logger
	next   http.Handler
}

// responseWriter is an implementation of the http.ResponseWriter interface that captures the
// response details that we want to write to the log.
type responseWriter struct {
	status int
	length int
	next   http.ResponseWriter
}

// NewHandlerWrapper creates a builder that can then be used to configure and create a logging HTTP
// handler wrapper.
func NewHandlerWrapper() *HandlerWrapperBuilder {
	return &HandlerWrapperBuilder{}
}

// Logger sets the logger that the wrapper and the wrapped HTTP handlers will use to write messages
// to the log.
func (b *HandlerWrapperBuilder) Logger(value sdklogging.Logger) *HandlerWrapperBuilder {
	b.logger = value
	return b
}

// Build uses the data stored in the builder to create a new logging HTTP handler wrapper.
func (b *HandlerWrapperBuilder) Build(ctx context.Context) (result *HandlerWrapper, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}

	// Create and populate the object:
	result = &HandlerWrapper{
		logger: b.logger,
	}

	return
}

// Wrap takes an HTTP handler and wraps it with another HTTP handler that writes to the log details
// of the requests and responses.
func (w *HandlerWrapper) Wrap(next http.Handler) http.Handler {
	return &handler{
		logger: w.logger,
		next:   next,
	}
}

// ServeHTTP is the implementation of the http.Handler interface.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get the context:
	ctx := r.Context()

	// Don't write to the log anything about request coming from probes:
	path := r.URL.Path
	agent := r.UserAgent()
	if path == probePath && agent == probeAgent {
		h.next.ServeHTTP(w, r)
		return
	}

	// Write the request details:
	h.logger.Info(ctx, "%s '%s'", r.Method, r.URL)

	// In order to write to the log the response details we need to replace the response writer
	// with one that intercepts the methods that set those details:
	writer := responseWriter{next: w}

	// Send the request to the next handler and measure the time it takes to receive
	// the response:
	start := time.Now()
	h.next.ServeHTTP(&writer, r)
	duration := time.Since(start)

	// Write the response details:
	h.logger.Info(
		ctx,
		"Took %dms, returning http %d, length %dB",
		int64(duration/time.Millisecond), writer.status, writer.length,
	)
}

// Header is part of the implementation of the http.ResponseWriter interface.
func (w *responseWriter) Header() http.Header {
	return w.next.Header()
}

// WriteHeader is part of the implementation of the http.ResponseWriter interface.
func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.next.WriteHeader(status)
}

// Write is part of the implementation of the http.ResponseWriter interface.
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.next.Write(b)
	w.length += n
	return n, err
}

// Details of the requests from probes, that should not be sent to the log:
const (
	probePath  = "/api/clusters_mgmt"
	probeAgent = "Probe"
)
