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

package helpers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/config"
)

func Serve(ctx context.Context, cfg *config.Listener,
	server *http.Server) error {
	// For Unix sockets check if the socket file already exists, and delete it. This is
	// necessary because in the production environment the container may be restarted by
	// Kubernetes, but the socket file will still exist, making the restart fail.
	if cfg.Network == "unix" {
		_, err := os.Stat(cfg.Address)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf(
				"can't check if Unix socket '%s' exists: %w",
				cfg.Address, err,
			)
		}
		if err == nil {
			err = os.Remove(cfg.Address)
			if err != nil {
				return fmt.Errorf(
					"can't delete existing Unix socket '%s': %w",
					cfg.Address, err,
				)
			}
		}
	}

	// Create the listener:
	listener, err := net.Listen(cfg.Network, cfg.Address)
	if err != nil {
		return fmt.Errorf(
			"can't create listener for network '%s' and address '%s': %w",
			cfg.Network, cfg.Address, err,
		)
	}

	// Start the server:
	if cfg.Certificate != "" && cfg.Key != "" {
		err = server.ServeTLS(listener, cfg.Certificate, cfg.Key)
	} else {
		err = server.Serve(listener)
	}
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server terminated with error: %v", err)
	}

	return nil
}
