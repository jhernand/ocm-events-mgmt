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

package logic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/segmentio/kafka-go"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/debezium"
	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/models"
)

// EventProducerBuilder contains the data and logic needed to create a new event producer.
type EventProducerBuilder struct {
	logger      logging.Logger
	install     bool
	brokers     []string
	tlsEnable   bool
	tlsCA       string
	tlsInsecure bool
	inputTopic  string
	inputGroup  string
	outputTopic string
}

// EventProducer knows how to produce events from raw database change notifications.
type EventProducer struct {
	logger logging.Logger
	reader *kafka.Reader
	writer *kafka.Writer
}

// NewEventProducer creates a builder that can then be used to configure and create a new event
// producer.
func NewEventProducer() *EventProducerBuilder {
	return &EventProducerBuilder{}
}

// Install sets or clears the flag that indicates if the producer should try to create the topics if
// needs before starting. Default value is false.
func (b *EventProducerBuilder) Install(value bool) *EventProducerBuilder {
	b.install = value
	return b
}

// Logger sets the logger that the producer will use to write to the log. This is mandatory.
func (b *EventProducerBuilder) Logger(value logging.Logger) *EventProducerBuilder {
	b.logger = value
	return b
}

// Brokers sets the Kafka brokers that the producer will use to read and write Kafka messages. This
// is mandatory.
func (b *EventProducerBuilder) Brokers(values []string) *EventProducerBuilder {
	b.brokers = append(b.brokers, values...)
	return b
}

// TLSEnable sets the flag that indicates if connections to the Kafka brokers should use TLS. Defaut
// is false.
func (b *EventProducerBuilder) TLSEnable(value bool) *EventProducerBuilder {
	b.tlsEnable = value
	return b
}

// TLSCA sets the certificate (in PEM format) of the authoritiy that should be used to verify the
// TLS certificates presented by the Kafka brokers. If no specified then the default system CAs will
// be used.
func (b *EventProducerBuilder) TLSCA(value string) *EventProducerBuilder {
	b.tlsCA = value
	return b
}

// TLSInsecure sets the flag that indicates if the certificates presented by the Kafka brokers should
// be verified. The default is false and shoundl't be changed for production environments.
func (b *EventProducerBuilder) TLSInsecure(value bool) *EventProducerBuilder {
	b.tlsInsecure = value
	return b
}

// InputTopic sets the Kafka topic where the producer will retrieve raw database change
// notificatinos. This is mandatory.
func (b *EventProducerBuilder) InputTopic(value string) *EventProducerBuilder {
	b.inputTopic = value
	return b
}

// InputGroup sets the Kafka consumer group that the producer will use to read database change
// notifications. This is mandatory.
func (b *EventProducerBuilder) InputGroup(value string) *EventProducerBuilder {
	b.inputGroup = value
	return b
}

// OutputTopic sets the Kafka topic where the producer will write the events.
func (b *EventProducerBuilder) OutputTopic(value string) *EventProducerBuilder {
	b.outputTopic = value
	return b
}

// Build uses the data stored in the builder to create a new event producer.
func (b *EventProducerBuilder) Build(ctx context.Context) (result *EventProducer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if len(b.brokers) == 0 {
		err = errors.New("at least one broker is mandatory")
		return
	}
	if b.inputTopic == "" {
		err = errors.New("input topic is mandatory")
		return
	}
	if b.inputGroup == "" {
		err = errors.New("input group is mandatory")
		return
	}
	if b.outputTopic == "" {
		err = errors.New("output topic is mandatory")
		return
	}

	// Make an alias for the logger so that we don't capture a reference to the builder in
	// closures or goroutines:
	logger := b.logger

	// Create the dialer:
	dialer, err := b.createDialer(ctx)
	if err != nil {
		return
	}

	// Try to create the topics if needed:
	if b.install {
		err = b.createTopics(ctx, dialer)
		if err != nil {
			return
		}
	}

	// Copy the list of brokers to avoid side effects if the slice passed to the builder
	// changes:
	brokers := make([]string, len(b.brokers))
	copy(brokers, b.brokers)

	// Create the bridges for the logging mechanism of the Kafka library:
	errorLogger := kafka.LoggerFunc(func(msg string, args ...interface{}) {
		logger.Error(ctx, msg, args...)
	})

	// Create the message reader:
	reader := kafka.NewReader(kafka.ReaderConfig{
		Dialer:      dialer,
		ErrorLogger: errorLogger,
		Brokers:     b.brokers,
		GroupTopics: []string{b.inputTopic},
		GroupID:     b.inputGroup,
	})

	// Create the message writer:
	writer := kafka.NewWriter(kafka.WriterConfig{
		Dialer:      dialer,
		ErrorLogger: errorLogger,
		Brokers:     b.brokers,
		Topic:       b.outputTopic,
	})

	// Create and populate the object:
	result = &EventProducer{
		logger: logger,
		reader: reader,
		writer: writer,
	}

	// Start the goroutine that processes messages:
	go result.loop()

	return
}

func (b *EventProducerBuilder) createDialer(ctx context.Context) (dialer *kafka.Dialer, err error) {
	// Start with the default configuration:
	dialer = &kafka.Dialer{}
	*dialer = *kafka.DefaultDialer

	// Configure TLS if needed:
	if b.tlsEnable {
		// Create CA certificate pool:
		var caPool *x509.CertPool
		if b.tlsCA != "" {
			caPool = x509.NewCertPool()
			if !caPool.AppendCertsFromPEM([]byte(b.tlsCA)) {
				err = fmt.Errorf("the TLS CA doesn't contain any certificate")
				return
			}
		} else {
			caPool, err = x509.SystemCertPool()
			if err != nil {
				return
			}
		}

		// Set the configuration:
		dialer.TLS = &tls.Config{
			RootCAs:            caPool,
			InsecureSkipVerify: b.tlsInsecure,
		}
	}

	return
}

func (b *EventProducerBuilder) createTopics(ctx context.Context, dialer *kafka.Dialer) error {
	// Create the client:
	client := &kafka.Client{
		Addr: kafka.TCP(b.brokers...),
		Transport: &kafka.Transport{
			TLS: dialer.TLS,
		},
	}

	// Prepare the list of topics:
	topics := []kafka.TopicConfig{
		{
			Topic:             b.inputTopic,
			NumPartitions:     -1,
			ReplicationFactor: -1,
		},
		{
			Topic:             b.outputTopic,
			NumPartitions:     -1,
			ReplicationFactor: -1,
		},
	}

	// Send the request to create the topics:
	response, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{
		Topics: topics,
	})
	if err != nil {
		return err
	}

	// Discard errors that indicate that topics already exist:
	count := 0
	for topic, err := range response.Errors {
		if err == nil {
			b.logger.Info(ctx, "Created topic '%s'", topic)
			continue
		}
		var kafkaErr kafka.Error
		if errors.As(err, &kafkaErr) && kafkaErr == kafka.TopicAlreadyExists {
			b.logger.Info(ctx, "Topic '%s' already exists", topic)
			continue
		}
		if err != nil {
			b.logger.Error(ctx, "Can't create topic '%s': %v", topic, err)
			count++
		}
	}

	// Return an error if there is any error that can't be ignored:
	if count > 0 {
		return fmt.Errorf("can't create topics")
	}

	return nil
}

func (p *EventProducer) loop() {
	for {
		ctx := context.Background()
		message, err := p.reader.ReadMessage(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			p.logger.Error(ctx, "Can't read message: %v", err)
			break
		}
		if p.logger.DebugEnabled() {
			value := message.Value
			var parsed interface{}
			err := json.Unmarshal(value, &parsed)
			if err == nil {
				value, err = json.MarshalIndent(parsed, "", "  ")
				if err != nil {
					value = message.Value
				}
			}
			p.logger.Debug(
				ctx,
				"Received message from topic '%s' with offset %d, "+
					"value follows:\n%s",
				message.Topic, message.Offset, string(value),
			)
		}
		err = p.processMessage(ctx, message)
		if err != nil {
			p.logger.Error(ctx, "Can't process message: %v", err)
		}
	}
}

func (p *EventProducer) processMessage(ctx context.Context, inMessage kafka.Message) error {
	// Parse the message as a Debezium event:
	var inEvent debezium.Event
	err := json.Unmarshal(inMessage.Value, &inEvent)
	if err != nil {
		return err
	}

	// Calculate the object kind:
	var change models.Change
	change.Kind = models.ChangeKind
	switch inEvent.Source.Table {
	case "clusters":
		change.Object.Kind = cmv1.ClusterLinkKind
	case "versions":
		change.Object.Kind = cmv1.VersionKind
	default:
		return fmt.Errorf(
			"don't know to to process change for table '%s'",
			inEvent.Source.Table,
		)
	}

	// Calculate the change type:
	switch inEvent.Op {
	case debezium.CreateOp:
		change.Type = models.ChangeTypeCreate
	case debezium.UpdateOp:
		change.Type = models.ChangeTypeUpdate
	case debezium.DeleteOp:
		change.Type = models.ChangeTypeDelete
	default:
		return fmt.Errorf(
			"don't know to to process operation '%s'",
			inEvent.Op,
		)
	}

	// Extract the object identifier:
	if change.Object.ID == "" && inEvent.After != nil {
		value, ok := inEvent.After["id"]
		if ok {
			change.Object.ID, ok = value.(string)
			if !ok {
				return fmt.Errorf(
					"expected 'after.id' field to be a string, but it "+
						"is of type '%T' and has value '%v'",
					value, value,
				)
			}
		}
	}
	if change.Object.ID == "" && inEvent.Before != nil {
		value, ok := inEvent.Before["id"]
		if ok {
			change.Object.ID, ok = value.(string)
			if !ok {
				return fmt.Errorf(
					"expected 'before.id' field to be a string, but it "+
						"is of type '%T' and has value '%v'",
					value, value,
				)
			}
		}
	}
	if change.Object.ID == "" {
		return fmt.Errorf("neither 'after.id' nor 'before.id' have a value")
	}

	// Create the output event:
	outEvent := models.Event{
		Kind:    models.EventKind,
		Details: change,
	}

	// Create the output message:
	outValue, err := json.Marshal(&outEvent)
	if err != nil {
		return err
	}
	outMessage := kafka.Message{
		Value: outValue,
	}

	// Write the output message:
	err = p.writer.WriteMessages(ctx, outMessage)
	if err != nil {
		return err
	}
	if p.logger.DebugEnabled() {
		value := outMessage.Value
		var parsed interface{}
		err := json.Unmarshal(value, &parsed)
		if err == nil {
			value, err = json.MarshalIndent(parsed, "", "  ")
			if err != nil {
				value = outMessage.Value
			}
		}
		p.logger.Debug(
			ctx,
			"Sent message to topic '%s', value follows:\n%s",
			p.writer.Topic, value,
		)
	}

	return nil
}

// Close release all the resources used by the producer.
func (p *EventProducer) Close() error {
	var err error
	err = p.reader.Close()
	if err != nil {
		return err
	}
	err = p.writer.Close()
	if err != nil {
		return err
	}
	return nil
}
