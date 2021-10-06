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

package server

import (
	"context"
	"fmt"
	"os"
	sgnl "os/signal"
	"syscall"

	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift-online/ocm-sdk-go/configuration"
	"github.com/spf13/cobra"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/logging"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/logic"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/api"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/health"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/servers/metrics"
)

var Cmd = &cobra.Command{
	Use:   "server",
	Short: "Runs the events management server",
	Long:  "Runs the events management server.",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	// Create a context:
	ctx := context.Background()

	// Create a logger with the default configuration that we can use till we have loaded the
	// configuration:
	logger, err := logging.NewLogger().
		Build(ctx)
	if err != nil {
		logger.Error(ctx, "Can't create default logger: %v", err)
		os.Exit(1)
	}

	// Check the command line:
	if len(args) == 0 {
		logger.Error(ctx, "At least one configuration source is required")
		os.Exit(1)
	}

	// Load the configuration:
	cfgBuilder := configuration.New()
	logger.Info(ctx, "Loading default configuration")
	cfgBuilder.Load(defaultConfig)
	for _, arg := range args {
		logger.Info(ctx, "Loading configuration from '%s'", arg)
		cfgBuilder.Load(arg)
	}
	cfgObject, err := cfgBuilder.Build()
	if err != nil {
		logger.Error(ctx, "Can't load configuration: %v", err)
		os.Exit(1)
	}
	var cfg configData
	err = cfgObject.Populate(&cfg)
	if err != nil {
		logger.Error(ctx, "Can't populate configuration: %v", err)
		os.Exit(1)
	}

	// Check the configuration:
	ok := true
	if cfg.Listeners.API == nil {
		logger.Error(ctx, "Parameter 'listeners.api' is mandatory")
		ok = false
	}
	if cfg.Listeners.Health == nil {
		logger.Error(ctx, "Parameter 'listeners.health' is mandatory")
		ok = false
	}
	if cfg.Listeners.Metrics == nil {
		logger.Error(ctx, "Parameter 'listeners.metrics' is mandatory")
		ok = false
	}
	if len(cfg.Kafka.Brokers) == 0 {
		logger.Error(ctx, "Parameter 'kafka.brokers' must have at least one value")
		ok = false
	}
	if cfg.Kafka.Topic == "" {
		logger.Error(ctx, "Parameter 'kafka.topic' is mandatory")
		ok = false
	}
	if !ok {
		os.Exit(1)
	}

	// Now that we have loaded the configuration we can replace the logger with one configured
	// according to that configuration:
	logger, err = logging.NewLogger().
		Level(cfg.Log.Level).
		Build(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't create configured logger: %v\n", err)
		os.Exit(1)
	}

	// Create the connection:
	logger.Info(ctx, "Creating connection")
	connection, err := sdk.NewConnectionBuilder().
		Logger(logger).
		Load(cfg.Connection).
		Build()
	if err != nil {
		logger.Error(ctx, "Can't create connection: %v", err)
		os.Exit(1)
	}

	// Create the service:
	logger.Info(ctx, "Creating service")
	serviceBuilder := logic.NewEventService()
	serviceBuilder.Logger(logger)
	serviceBuilder.Install(cfg.Install)
	serviceBuilder.Connection(connection)
	serviceBuilder.Brokers(cfg.Kafka.Brokers)
	if cfg.Kafka.TLS != nil {
		serviceBuilder.TLSEnable(true)
		serviceBuilder.TLSCA(cfg.Kafka.TLS.CA)
		serviceBuilder.TLSInsecure(cfg.Kafka.TLS.Insecure)
	}
	serviceBuilder.Topic(cfg.Kafka.Topic)
	serviceObject, err := serviceBuilder.Build(ctx)
	if err != nil {
		logger.Error(ctx, "Can't create event service: %v", err)
		os.Exit(1)
	}

	// Start the API server:
	logger.Info(ctx, "Starting API server at '%s'", cfg.Listeners.API.Address)
	apiServer, err := api.NewServer().
		Logger(logger).
		Listener(cfg.Listeners.API).
		EventService(serviceObject).
		AuthKeysFile(cfg.Auth.JWKSFile).
		AuthKeysURL(cfg.Auth.JWKSURL).
		Build(ctx)
	if err != nil {
		logger.Error(ctx, "Can't create API server: %v", err)
		os.Exit(1)
	}

	// Start the health server:
	logger.Info(ctx, "Starting health server at '%s'", cfg.Listeners.Health.Address)
	healthServer, err := health.NewServer().
		Logger(logger).
		Listener(cfg.Listeners.Health).
		Build(ctx)
	if err != nil {
		logger.Error(ctx, "Can't create health server: %v", err)
		os.Exit(1)
	}

	// Start the metrics server:
	logger.Info(ctx, "Starting metrics server at '%s'", cfg.Listeners.Metrics.Address)
	metricsServer, err := metrics.NewServer().
		Logger(logger).
		Listener(cfg.Listeners.Metrics).
		Build(ctx)
	if err != nil {
		logger.Error(ctx, "Can't create metrics server: %v", err)
		os.Exit(1)
	}

	// Wait till we receive a signal:
	logger.Info(ctx, "Waiting for signal to stop")
	signals := make(chan os.Signal, 1)
	sgnl.Notify(signals, syscall.SIGTERM)
	sgnl.Notify(signals, syscall.SIGINT)
	signal := <-signals
	logger.Info(ctx, "Received signal '%s', will stop", signal)

	// Close the components:
	logger.Info(ctx, "Closing event service")
	err = serviceObject.Close()
	if err != nil {
		logger.Error(ctx, "Can't close event service: %v", err)
	}
	logger.Info(ctx, "Closing API server")
	err = apiServer.Close()
	if err != nil {
		logger.Error(ctx, "Can't close API server: %v", err)
	}
	logger.Info(ctx, "Closing health server")
	err = healthServer.Close()
	if err != nil {
		logger.Error(ctx, "Can't close health server: %v", err)
	}
	logger.Info(ctx, "Closing metrics server")
	err = metricsServer.Close()
	if err != nil {
		logger.Error(ctx, "Can't close health server: %v", err)
	}
}
