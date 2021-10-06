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

package logging

// This file contains tests for the logger.

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	. "github.com/onsi/ginkgo"                  // nolint
	. "github.com/onsi/ginkgo/extensions/table" // nolint
	. "github.com/onsi/gomega"                  // nolint
)

var _ = Describe("Logger", func() {
	var ctx context.Context
	var file string

	BeforeEach(func() {
		var err error

		// Create a context:
		ctx = context.Background()

		// Create a temporary file to write the log. Note that we just need the name, the
		// file doesn't need to be open as the logger will open it.
		fd, err := ioutil.TempFile("", "test-*.log")
		Expect(err).ToNot(HaveOccurred())
		file = fd.Name()
		fd.Close()
	})

	AfterEach(func() {
		var err error

		// Remote the temporary log file:
		if file != "" {
			err = os.Remove(file)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	DescribeTable(
		"Generates the expected line format",
		func(write func(context.Context, *Logger), level, message string) {
			// Add an identifier to the context:
			type key int
			id := uuid.NewString()
			ctx = context.WithValue(ctx, key(0), id)

			// Create the logger:
			logger, err := NewLogger().
				File(file).
				Level("debug").
				Field("id", func(ctx context.Context) string {
					return ctx.Value(key(0)).(string)
				}).
				Build(ctx)
			Expect(err).ToNot(HaveOccurred())

			// Write a message:
			write(ctx, logger)

			// Get the output line:
			data, err := ioutil.ReadFile(file)
			Expect(err).ToNot(HaveOccurred())
			parts := strings.SplitN(string(data), " ", 5)
			Expect(parts).To(HaveLen(5))

			// Check the timestamp:
			timestamp, err := time.Parse(time.RFC3339, parts[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(time.Since(timestamp).Milliseconds()).To(BeNumerically("<", 1000))

			// Check the level:
			Expect(parts[1]).To(Equal(level))

			// Check the source file and line number:
			Expect(parts[2]).To(MatchRegexp(`^.*\.go:\d+$`))

			// Check the operation identifier:
			Expect(parts[3]).To(MatchRegexp(`\[id='.*'\]`))

			// Check the message:
			Expect(parts[4]).To(MatchRegexp(message))
		},
		Entry(
			"Debug",
			func(ctx context.Context, logger *Logger) {
				logger.Debug(ctx, "Hello!")
			},
			"DEBUG",
			`^Hello!\n$`,
		),
		Entry(
			"Info",
			func(ctx context.Context, logger *Logger) {
				logger.Info(ctx, "Hello!")
			},
			"INFO",
			`^Hello!\n$`,
		),
		Entry(
			"Warn",
			func(ctx context.Context, logger *Logger) {
				logger.Warn(ctx, "Hello!")
			},
			"WARN",
			`^Hello!\n$`,
		),
		Entry(
			"Error, including stack trace",
			func(ctx context.Context, logger *Logger) {
				logger.Error(ctx, "Hello!")
			},
			"ERROR",
			`^Hello!\ngitlab.cee.redhat.com/`,
		),
		Entry(
			"Format and arguments",
			func(ctx context.Context, logger *Logger) {
				logger.Info(ctx, "Hello %s!", "Mary")
			},
			"INFO",
			`^Hello Mary!\n$`,
		),
	)

	It("Honors the level", func() {
		// Create the logger:
		logger, err := NewLogger().
			File(file).
			Level("info").
			Build(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Write a message:
		logger.Debug(ctx, "Hello!")

		// Check the result:
		data, err := ioutil.ReadFile(file)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(BeEmpty())
	})

	It("Rejects wrong level", func() {
		// Create the logger:
		logger, err := NewLogger().
			File(file).
			Level("junk").
			Build(ctx)
		Expect(err).To(HaveOccurred())
		Expect(logger).To(BeNil())
		message := err.Error()
		Expect(message).To(ContainSubstring("level"))
		Expect(message).To(ContainSubstring("junk"))
	})
})
