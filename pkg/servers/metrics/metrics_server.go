package metrics

import (
	"context"
	"errors"
	"net/http"

	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/config"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/helpers"
)

// ServerBuilder contains the data and logic needed to create a new metrics server.
type ServerBuilder struct {
	logger   logging.Logger
	listener *config.Listener
}

// Server serves Prometheus metrics.
type Server struct {
	logger     logging.Logger
	httpServer *http.Server
}

// NewServer creates a builder that can then be used to configure and create a new metrics server.
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
		err = errors.New("logger is mandatory")
		return
	}
	if b.listener == nil {
		err = errors.New("listener is mandatory")
		return
	}

	// Create and populate the object:
	server := &Server{
		logger: b.logger,
	}

	// Create the nandler:
	handler := promhttp.Handler()

	// Create the servers with support for HTTP2 prior knowledge:
	http2Server := &http2.Server{}
	server.httpServer = &http.Server{
		Handler: h2c.NewHandler(handler, http2Server),
	}

	// Start the server:
	go func() {
		err := helpers.Serve(ctx, b.listener, server.httpServer)
		if err != nil {
			b.logger.Error(ctx, "Server finished with error: %v", err)
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
