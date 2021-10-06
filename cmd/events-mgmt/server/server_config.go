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
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/config"
)

// configData contains the configuration of the server.
type configData struct {
	// Log contains the configuration of the log.
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`

	// Install indicates if the producer should try to create the Kafka topics that it needs.
	// This is intended for development environments, specially when the broker isn't
	// persistent. In production environments this should be set to false and the topics should
	// be created in advance before starting the producer.
	Install bool `yaml:"install"`

	// Listeners contains the configuration of the network listeners.
	Listeners struct {
		API     *config.Listener `yaml:"api"`
		Health  *config.Listener `yaml:"health"`
		Metrics *config.Listener `yaml:"metrics"`
	} `yaml:"listeners"`

	// Autho contains the settings used to authenticate incoming requests.
	Auth struct {
		JWKSFile string `yaml:"jwks_file"`
		JWKSURL  string `yaml:"jwks_url"`
	} `yaml:"auth"`

	// Connection is the configuration of the `api.openshift.com` connection that the server
	// will use to talk to other services.
	Connection interface{} `yaml:"connection"`

	/// Kafka contains the settings to connect to the Kafka broker.
	Kafka struct {
		Brokers []string    `yaml:"brokers"`
		TLS     *config.TLS `yaml:"tls"`
		Topic   string      `yaml:"topic"`
	} `yaml:"kafka"`
}

// defaultConfig contains the default configuration values.
const defaultConfig = `
# Log settings:
log:
  level: info

# Indicates if the producer should try to create the Kafka topics that it needs.
# This is intended for development environments, specially when the broker isn't
# persistent. In production environments this should be set to false and the
# topics should be created in advance before starting the producer.
install: false

# Listener settings:
listeners:
  api:
    network: tcp
    address: :8000
  health:
    network: tcp
    address: :8001
  metrics:
    network: tcp
    address: :8002

# Authentication settings:
auth:
  jwks_url: https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs
`
