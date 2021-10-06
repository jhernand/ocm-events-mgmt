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

// ChangeKind is the value of the `kind` field that identifies objects as changes.
const ChangeKind = "Change"

// ChangeType representst the different types that can happen to an object: create, update and delete.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
)

// Change represents a change to an object.
type Change struct {
	// Kind identifies the type of the object. It will always be `Change`.
	Kind string `json:"kind,omitempty"`

	// Type indicates the type of change: create, update or delete.
	Type ChangeType `json:"type,omitempty"`

	// Object is a link to the changed object.
	Object Link `json:"object,omitempty"`
}
