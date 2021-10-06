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

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/openshift-online/ocm-sdk-go/authentication"
	"github.com/openshift-online/ocm-sdk-go/errors"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/openshift-online/ocm-sdk-go/metrics"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/api"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/config"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/logic"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/helpers"
)

// ServerBuilder contains the data and logic needed to create a new API server.
type ServerBuilder struct {
	logger       logging.Logger
	listener     *config.Listener
	authKeysFile string
	authKeysURL  string
	eventService *logic.EventService
}

// Server is a health check server.
type Server struct {
	logger       logging.Logger
	eventService *logic.EventService
	httpServer   *http.Server
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

// AuthKeysFile sets the file containing the keys that will be used to verify JSON web tokens.
func (b *ServerBuilder) AuthKeysFile(value string) *ServerBuilder {
	b.authKeysFile = value
	return b
}

// AuthKeysURL sets the URL containing the keys that will be used to verify JSON web tokens.
func (b *ServerBuilder) AuthKeysURL(value string) *ServerBuilder {
	b.authKeysURL = value
	return b
}

// Listener sets the configuration of the listener for the server. This is mandatory.
func (b *ServerBuilder) Listener(value *config.Listener) *ServerBuilder {
	b.listener = value
	return b
}

// EventService sets the service that will be used to manage events.
func (b *ServerBuilder) EventService(value *logic.EventService) *ServerBuilder {
	b.eventService = value
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
	if b.eventService == nil {
		err = fmt.Errorf("event service is mandatory")
		return
	}

	// Create the server:
	server := &Server{
		logger:       b.logger,
		eventService: b.eventService,
	}

	// Create the routes:
	handler, err := b.createRoutes(ctx, server)
	if err != nil {
		return
	}

	// Add the metrics handler:
	metricsWrapper, err := metrics.NewHandlerWrapper().
		Subsystem("api_inbound").
		Build()
	if err != nil {
		return
	}
	handler = metricsWrapper.Wrap(handler)

	// Add the compression handler:
	handler = handlers.CompressHandler(handler)

	// Add the cleanup handler:
	handler = api.CleanupMiddleware(handler)

	// Enable CORS support:
	handler = handlers.CORS(
		handlers.AllowedOrigins([]string{
			// OCM UI local development URLs
			"https://qa.foo.redhat.com:1337",
			"https://prod.foo.redhat.com:1337",
			"https://ci.foo.redhat.com:1337",
			"https://cloud.redhat.com",   // Production / candidate
			"https://console.redhat.com", // Production / candidate
			// Staging and test environments
			"https://qaprodauth.cloud.redhat.com",
			"https://qaprodauth.console.redhat.com",
			"https://qa.cloud.redhat.com",
			"https://qa.console.redhat.com",
			"https://ci.cloud.redhat.com",
			"https://ci.console.redhat.com",
			"https://cloud.stage.redhat.com",
			"https://console.stage.redhat.com",
			// API docs UI
			"https://api.stage.openshift.com",
			"https://api.openshift.com",
			// Customer portal
			"https://access.qa.redhat.com",
			"https://access.stage.redhat.com",
			"https://access.redhat.com",
		}),
		handlers.AllowedMethods([]string{
			http.MethodDelete,
			http.MethodGet,
			http.MethodPatch,
			http.MethodPost,
		}),
		handlers.AllowedHeaders([]string{
			"Authorization",
			"Content-Type",
		}),
		handlers.MaxAge(int((10 * time.Minute).Seconds())),
	)(handler)

	// Add the authentication handler:
	handler, err = authentication.NewHandler().
		Logger(b.logger).
		Public(fmt.Sprintf("^%s/%s/?$", api.Prefix, api.ID)).
		Public(fmt.Sprintf("^%s/%s/%s/?$", api.Prefix, api.ID, api.VersionID)).
		Public(fmt.Sprintf("^%s/%s/%s/openapi/?$", api.Prefix, api.ID, api.VersionID)).
		KeysFile(b.authKeysFile).
		KeysURL(b.authKeysURL).
		Next(handler).
		Build()
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
			b.logger.Warn(ctx, "API server finished with error: %v", err)
		}
	}()

	// Return the server:
	result = server
	return
}

func (b *ServerBuilder) createRoutes(ctx context.Context, s *Server) (result http.Handler,
	err error) {
	// Create the `/` router:
	rootRouter := mux.NewRouter()
	rootRouter.NotFoundHandler = http.HandlerFunc(errors.SendNotFound)

	// Create the `/api` router:
	apiRouter := rootRouter.PathPrefix(fmt.Sprintf("%s/%s", api.Prefix, api.ID)).Subrouter()
	apiRouter.HandleFunc("", api.SendServiceMetadata).Methods(http.MethodGet)

	// Create the `/api/v1` router:
	apiV1Router := apiRouter.PathPrefix(fmt.Sprintf("/%s", api.VersionID)).Subrouter()
	apiV1Router.HandleFunc("", api.SendVersionMetadata).Methods(http.MethodGet)
	apiV1Router.HandleFunc("/openapi", s.getOpenAPI).Methods(http.MethodGet)
	apiV1Router.HandleFunc("/events", s.listEvents).Methods(http.MethodGet)

	// Return the result:
	result = rootRouter
	return
}

// Close stops the server and releases all the resources it is using.
func (s *Server) Close() error {
	return s.httpServer.Shutdown(context.Background())
}
