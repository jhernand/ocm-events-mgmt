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

package health

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift-online/ocm-sdk-go/errors"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/config"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/helpers"
)

// ServerBuilder contains the data and logic needed to create a new health check server.
type ServerBuilder struct {
	logger   logging.Logger
	listener *config.Listener
}

// Server is a health check server.
type Server struct {
	logger     logging.Logger
	httpServer *http.Server
}

// NewServer creates a builder that can then be used to configure and create a new health check server.
func NewServer() *ServerBuilder {
	return &ServerBuilder{}
}

// Logger sets the logger that the server will use to write to the log. This is mandatory.
func (b *ServerBuilder) Logger(value logging.Logger) *ServerBuilder {
	b.logger = value
	return b
}

// Listener sets the configuration of the listener for the server. This is mandatory.
func (b *ServerBuilder) Listener(value *config.Listener) *ServerBuilder {
	b.listener = value
	return b
}

// Build uses the configuration stored in the builder to create a new health check server.
func (b *ServerBuilder) Build(ctx context.Context) (result *Server, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}
	if b.listener == nil {
		err = fmt.Errorf("listener is mandatory")
		return
	}

	// Create and populate the object:
	server := &Server{
		logger: b.logger,
	}

	// Create the routes:
	handler, err := b.createRoutes(ctx, result)
	if err != nil {
		return
	}

	// Create the servers with support for HTTP2 prior knowledge:
	http2Server := &http2.Server{}
	server.httpServer = &http.Server{
		Handler: h2c.NewHandler(handler, http2Server),
	}

	// Start the server:
	go func() {
		err = helpers.Serve(ctx, b.listener, server.httpServer)
		if err != nil {
			b.logger.Warn(ctx, "Health server finished with error: %v", err)
		}
	}()

	// Return the server:
	result = server
	return
}

// Close stops the server and releases all the resources it is using.
func (s *Server) Close() error {
	return s.httpServer.Shutdown(context.Background())
}

func (b *ServerBuilder) createRoutes(ctx context.Context, s *Server) (result http.Handler,
	err error) {
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(errors.SendNotFound)
	router.HandleFunc("/health", s.getHealth).Methods(http.MethodGet)
	result = router
	return
}
