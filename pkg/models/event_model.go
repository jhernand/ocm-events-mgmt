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

package models

// EventKind is the value of the `kind` field that identifies objects as events.
const EventKind = "Event"

// Event represents an event as published to Kafka.
type Event struct {
	// Kind identifies the type of the object. It will always be `Event`.
	Kind string `json:"kind,omitempty"`

	// ID is the unique identifier of the event. Note that this field will not be present in
	// actual Kafka messages, it will instead be calculated from the offset.
	ID string `json:"id,omitempty"`

	// Details contains optional additional details about the event.
	Details interface{} `json:"details,omitempty"`
}
