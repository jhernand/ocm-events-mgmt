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

// This file contains the API metadata types used by the service.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openshift-online/ocm-sdk-go/errors"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/info"
)

// Link represents a link.
type Link struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
	HREF string `json:"href,omitempty"`
}

// ServiceMetadataKind is the name of the type used to represent service meta-data.
const ServiceMetadataKind = "ServiceMetadata"

// ServiceMetadata represents the meta-data of the API.
type ServiceMetadata struct {
	Kind string `json:"kind,omitempty"`
	HREF string `json:"href,omitempty"`
	V1   *Link  `json:"v1,omitempty"`
}

// VersionMetadataKind is the name of the type used to represent version meta-data.
const VersionMetadataKind = "VersionMetadata"

// VersionMetadata represents the meta-data of a version of the API.
type VersionMetadata struct {
	Kind string `json:"kind,omitempty"`
	HREF string `json:"href,omitempty"`

	// ServerVersion is the version of the server. In development builds this will usually be
	// the Unix time. In production environments it will be the short git hash of the source.
	ServerVersion string `json:"server_version,omitempty"`

	// Link to the OpenAPI specification:
	OpenAPI *Link `json:"openapi,omitempty"`

	// Links to collections and sub-resources:
	Events *Link `json:"events,omitempty"`
}

// SendServiceMetadata sends the service metadata.
func SendServiceMetadata(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	body := ServiceMetadata{
		Kind: ServiceMetadataKind,
		HREF: fmt.Sprintf("%s/%s", Prefix, ID),
		V1: &Link{
			Kind: VersionMetadataKind,
			ID:   VersionID,
			HREF: fmt.Sprintf("%s/%s/%s", Prefix, ID, VersionID),
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		errors.SendPanic(w, r)
		return
	}

	// Send the response:
	w.Write(data)
}

// SendVersionMetadata sends version metadata response.
func SendVersionMetadata(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	body := VersionMetadata{
		HREF:          fmt.Sprintf("%s/%s/%s", Prefix, ID, VersionID),
		Kind:          VersionMetadataKind,
		ServerVersion: info.Version,
		OpenAPI: &Link{
			Kind: "OpenAPILink",
			HREF: fmt.Sprintf("%s/%s/%s/openapi", Prefix, ID, VersionID),
		},
		Events: &Link{
			Kind: "EventsLink",
			HREF: fmt.Sprintf("%s/%s/%s/events", Prefix, ID, VersionID),
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		errors.SendPanic(w, r)
		return
	}

	// Send the response:
	w.Write(data)
}
