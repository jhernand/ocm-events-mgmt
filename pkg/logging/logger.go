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

// This file contains an object that implements to logger interface used by the project.

package logging

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerBuilder contains the data and logic needed to build a logger that uses the zap library.
type LoggerBuilder struct {
	level  string
	file   string
	fields map[string]interface{}
	dsn    string
}

// Logger is a logger that uses the zap library.
type Logger struct {
	logger *zap.Logger
	fields map[string]interface{}
}

// NewLogger creates a builder that can then be used to configure a logger that uses the zap
// library.
func NewLogger() *LoggerBuilder {
	return &LoggerBuilder{
		fields: map[string]interface{}{},
	}
}

// File sets the output file. Default is to write to the standard output of the process.
func (b *LoggerBuilder) File(value string) *LoggerBuilder {
	b.file = value
	return b
}

// DSN sets the DSN that will be used to communicate with Sentry.
func (b *LoggerBuilder) DSN(value string) *LoggerBuilder {
	b.dsn = value
	return b
}

// Field adds a field that will be extracted from the context and added to the log entries. If the
// value is a function then it will be called to obtain the value, otherwise the value itself will
// be added.
func (b *LoggerBuilder) Field(name string, value interface{}) *LoggerBuilder {
	b.fields[name] = value
	return b
}

// Level sets the log level.
func (b *LoggerBuilder) Level(value string) *LoggerBuilder {
	b.level = value
	return b
}

// Build creates a new logger using the configuration stored in the builder.
func (b *LoggerBuilder) Build(ctx context.Context) (result *Logger, err error) {
	// Prepare the logger configuration:
	config := zap.NewProductionConfig()
	config.Encoding = EncoderName

	// Set the output file:
	file := "stdout"
	if b.file != "" {
		file = b.file
	}
	config.OutputPaths = []string{
		file,
	}

	// Set the log level:
	if b.level != "" {
		var level zapcore.Level
		err = level.UnmarshalText([]byte(b.level))
		if err != nil {
			return
		}
		config.Level = zap.NewAtomicLevelAt(level)
	}

	// Create the logger:
	logger, err := config.Build(
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return
	}

	// Copy the context field extractors:
	fields := map[string]interface{}{}
	for name, value := range b.fields {
		fields[name] = value
	}

	// Create and populate the object:
	result = &Logger{
		logger: logger,
		fields: fields,
	}

	return
}

// DebugEnabled returns true iff the debug level is enabled.
func (l *Logger) DebugEnabled() bool {
	return l.logger.Core().Enabled(zapcore.DebugLevel)
}

// InfoEnabled returns true iff the information level is enabled.
func (l *Logger) InfoEnabled() bool {
	return l.logger.Core().Enabled(zapcore.InfoLevel)
}

// WarnEnabled returns true iff the warning level is enabled.
func (l *Logger) WarnEnabled() bool {
	return l.logger.Core().Enabled(zapcore.WarnLevel)
}

// ErrorEnabled returns true iff the error level is enabled.
func (l *Logger) ErrorEnabled() bool {
	return l.logger.Core().Enabled(zapcore.ErrorLevel)
}

// Debug sends to the log a debug message.
func (l *Logger) Debug(ctx context.Context, format string, args ...interface{}) {
	fields := l.extractFields(ctx)
	l.write(ctx, fields, zapcore.DebugLevel, format, args)
}

// Info sends to the log an information message.
func (l *Logger) Info(ctx context.Context, format string, args ...interface{}) {
	fields := l.extractFields(ctx)
	l.write(ctx, fields, zapcore.InfoLevel, format, args)
}

// Warn sends to the log a warning message.
func (l *Logger) Warn(ctx context.Context, format string, args ...interface{}) {
	fields := l.extractFields(ctx)
	l.write(ctx, fields, zapcore.WarnLevel, format, args)
}

// Error sends to the log an error message.
func (l *Logger) Error(ctx context.Context, format string, args ...interface{}) {
	fields := l.extractFields(ctx)
	l.write(ctx, fields, zapcore.ErrorLevel, format, args)
}

// Fatal sends to the log an error message and then exits the process.
func (l *Logger) Fatal(ctx context.Context, format string, args ...interface{}) {
	fields := l.extractFields(ctx)
	l.write(ctx, fields, zapcore.FatalLevel, format, args)
}

func (l *Logger) extractFields(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	values := map[string]interface{}{}
	for name, value := range l.fields {
		switch field := value.(type) {
		case func() string:
			text := field()
			if text != "" {
				values[name] = text
			}
		case func() (string, bool):
			text, ok := field()
			if ok {
				values[name] = text
			}
		case func() (string, error):
			text, err := field()
			if err != nil {
				values[name] = text
			}
		case func(context.Context) string:
			text := field(ctx)
			if text != "" {
				values[name] = text
			}
		case func(context.Context) (string, bool):
			text, ok := field(ctx)
			if ok {
				values[name] = text
			}
		case func(context.Context) (string, error):
			text, err := field(ctx)
			if err != nil {
				values[name] = text
			}
		default:
			text := fmt.Sprintf("%s", value)
			if text != "" {
				values[name] = text
			}
		}
	}
	return values
}

func (l *Logger) write(ctx context.Context, fields map[string]interface{},
	level zapcore.Level, format string, args []interface{}) {
	list := make([]zap.Field, len(fields))
	i := 0
	for name, value := range fields {
		list[i] = zap.Any(name, value)
		i++
	}
	message := fmt.Sprintf(format, args...)
	switch level {
	case zapcore.DebugLevel:
		l.logger.Debug(message, list...)
	case zapcore.InfoLevel:
		l.logger.Info(message, list...)
	case zapcore.WarnLevel:
		l.logger.Warn(message, list...)
	case zapcore.ErrorLevel:
		l.logger.Error(message, list...)
	case zapcore.FatalLevel:
		l.logger.Fatal(message, list...)
	}
}

// Names of well known fields:
const (
	userField string = "user"
)
