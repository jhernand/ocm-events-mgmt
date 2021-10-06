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

// This file contains the API types used to represent events.

package api

// EventKind is the name of the type used to represent events.
const EventKind = "Event"

// Event respresents an event.
type Event struct {
	// Kind identifies the type of this object. It will always be `Event`.
	Kind *string `json:"kind,omitempty"`

	// ID is the unique identifier of the event.
	ID *string `json:"id,omitempty"`

	// HREF is the absolute location of the event in the URL space of the API.
	HREF *string `json:"href,omitempty"`

	// Details contains optional additional details about the event.
	Details interface{} `json:"details,omitempty"`
}
