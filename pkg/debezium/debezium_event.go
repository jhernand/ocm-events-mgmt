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

package debezium

// Event represents a change event as published to Kafka by the Debezium PostgreSQL plugin. Note
// that this is only a partial representiation, ignoring all the pieces that we don't need for now,
// like the schema.
type Event struct {
	// Source is the source of the event.
	Source Source `json:"source"`

	// Op is the type of operation: insert, update or delete.
	Op Op `json:"op"`

	// Before contains the row data before the event.
	Before map[string]interface{} `json:"before"`

	// After contains the row data after the event.
	After map[string]interface{} `json:"after"`
}

// Source describes the source of the event.
type Source struct {
	// Table is the name of the source table.
	Table string `json:"table"`

	// TsMs is the timestamp of operation.
	TsMs uint64 `json:"ts_ms"`
}

// Op represents the types of operations that can be performed on a row.
type Op string

const (
	// CeateOp represents a row insert.
	CreateOp Op = "c"

	// UpdateOp represents a row update.
	UpdateOp Op = "u"

	// DeleteOp represents a row delete.
	DeleteOp Op = "d"
)
