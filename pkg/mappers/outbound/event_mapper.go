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

package outbound

import (
	"context"
	"fmt"

	"github.com/openshift-online/ocm-sdk-go/helpers"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/api"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/models"
)

// MapEvent transforms an event model as received from Kafka into the event representation expected
// by API users.
func MapEvent(ctx context.Context, in *models.Event) *api.Event {
	if in == nil {
		return nil
	}
	out := &api.Event{}
	out.Kind = helpers.NewString(api.EventKind)
	if in.ID != "" {
		out.ID = helpers.NewString(in.ID)
		out.HREF = helpers.NewString(fmt.Sprintf(
			"%s/%s/%s/events/%s",
			api.Prefix,
			api.ID,
			api.VersionID,
			in.ID,
		))
	}
	out.Details = in.Details
	return out
}
