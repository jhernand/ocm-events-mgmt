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
	"encoding/json"
	"net/http"

	"github.com/openshift-online/ocm-sdk-go/errors"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/logic"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/mappers/outbound"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/models"
)

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	// Get the context:
	ctx := r.Context()

	// Check that we can flush the writer:
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.Error(
			ctx,
			"Writer of type '%T' doesn't implement the flusher interface",
			w,
		)
		errors.SendInternalServerError(w, r)
		return
	}

	// Get the request variables:
	from := r.URL.Query().Get("from")

	// Send the headers:
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Create the JSON encoder:
	encoder := json.NewEncoder(w)

	// List the events:
	err := s.eventService.List(ctx, logic.EventListOptions{
		From: from,
		Callback: func(model *models.Event) error {
			event := outbound.MapEvent(ctx, model)
			err := encoder.Encode(event)
			if err != nil {
				return err
			}
			flusher.Flush()
			return nil
		},
	})
	if err != nil {
		s.logger.Error(ctx, "Can't list events: %v", err)
		return
	}
}

func (s *Server) getOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// TODO: Replace the empty string with the OpenAPI specification of the service
	// from the SDK.
	_, err := w.Write([]byte(""))
	if err != nil {
		s.logger.Error(r.Context(), "Write to client: %s", err)
	}
}
