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

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/cmd/events-mgmt/producer"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/cmd/events-mgmt/server"
)

var root = &cobra.Command{
	Use:  "events-mgmt",
	Long: "Events management service.",
}

func init() {
	root.AddCommand(server.Cmd)
	root.AddCommand(producer.Cmd)
}

func main() {
	root.SetArgs(os.Args[1:])
	err := root.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't execute root command: %v\n", err)
		os.Exit(1)
	}
}
