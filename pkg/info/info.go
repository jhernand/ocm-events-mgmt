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

// This file contains information about the service, like the version.

package info

// Version is the version of the project. This will be populated by the Go linker with an option
// similar to this:
//
//	-ldflags="-X gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/info.Version=123"
//
// It will then be returned by the API in the `server_version` attribute of the version metadata,
// and will also be used in other places that need the version.
var Version string
