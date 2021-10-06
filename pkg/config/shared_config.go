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

package config

// Listener contains the configuration of a network listener.
type Listener struct {
	// Network is the network for the listener. Valid values are 'tcp' and 'unix'.
	Network string `yaml:"network"`

	// Address is the address for the listener. For example ':8000' or '0.0.0.0:8000' for a TCP
	// listener and '/tmp/my.socket' for a Unix listener. Note that this is not a URL.
	Address string `yaml:"address"`

	// Certificate is the name of a PEM file containing the certificate used to configure TLS on
	// the listener.
	Certificate string `yaml:"certificate"`

	// Key pair is the name of a PEM file containing the private key used to configure TLS on
	// the listener.
	Key string `yaml:"key"`
}

// TLS contains the configuration used to connect to a TLS server.
type TLS struct {
	// Insecure indicates if the certificate and host name of the TLS server should be verified.
	// In production environments this should be false.
	Insecure bool `yaml:"insecure"`

	// CA is the PEM encoded certificate of the authority that will be used to verify TLS
	// certificate presented by the server.
	CA string `yaml:"ca"`
}
