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
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/golang-jwt/jwt/v4"
	sdk "github.com/openshift-online/ocm-sdk-go"
	amv1 "github.com/openshift-online/ocm-sdk-go/accountsmgmt/v1"
	"github.com/openshift-online/ocm-sdk-go/authentication"
	azv1 "github.com/openshift-online/ocm-sdk-go/authorizations/v1"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift-online/ocm-sdk-go/data"
	"github.com/openshift-online/ocm-sdk-go/logging"
	"github.com/segmentio/kafka-go"

	"gitlab.cee.redhat.com/service/ocm-events-mgmt/pkg/models"
)

// EventServiceBuilder contains the data and logic needed to create a new event service.
type EventServiceBuilder struct {
	logger      logging.Logger
	install     bool
	connection  *sdk.Connection
	brokers     []string
	tlsEnable   bool
	tlsCA       string
	tlsInsecure bool
	topic       string
}

// EventService is a service that knows how to manage events.
type EventService struct {
	logger       logging.Logger
	errorLogger  kafka.Logger
	accessReview *azv1.AccessReviewClient
	brokers      []string
	topic        string
	offsetCipher cipher.Block
	dataDigger   *data.Digger
	dialer       *kafka.Dialer
}

// NewEventService creates a builder that can then be used to configure and create a new event
// service.
func NewEventService() *EventServiceBuilder {
	return &EventServiceBuilder{}
}

// Logger sets the logger that the service will use to write to the log. This is mandatory.
func (b *EventServiceBuilder) Logger(value logging.Logger) *EventServiceBuilder {
	b.logger = value
	return b
}

// Install sets or clears the flag that indicates if the service should try to create the topics if
// needs before starting. Default value is false.
func (b *EventServiceBuilder) Install(value bool) *EventServiceBuilder {
	b.install = value
	return b
}

// Connection sets the `api.openshift.com` connection that the service will use to check
// permissions. This is mandatory.
func (b *EventServiceBuilder) Connection(value *sdk.Connection) *EventServiceBuilder {
	b.connection = value
	return b
}

// Brokers sets the Kafka brokers that the service will use to retrieve events. This is mandatory.
func (b *EventServiceBuilder) Brokers(value []string) *EventServiceBuilder {
	b.brokers = append(b.brokers, value...)
	return b
}

// TLSEnable sets the flag that indicates if connections to the Kafka brokers should use TLS. Defaut
// is false.
func (b *EventServiceBuilder) TLSEnable(value bool) *EventServiceBuilder {
	b.tlsEnable = value
	return b
}

// TLSCA sets the certificate (in PEM format) of the authoritiy that should be used to verify the
// TLS certificates presented by the Kafka brokers. If no specified then the default system CAs will
// be used.
func (b *EventServiceBuilder) TLSCA(value string) *EventServiceBuilder {
	b.tlsCA = value
	return b
}

// TLSInsecure sets the flag that indicates if the certificates presented by the Kafka brokers should
// be verified. The default is false and shoundl't be changed for production environments.
func (b *EventServiceBuilder) TLSInsecure(value bool) *EventServiceBuilder {
	b.tlsInsecure = value
	return b
}

// Topic sets the Kafka topic where the service will retrieve events. This is mandatory.
func (b *EventServiceBuilder) Topic(value string) *EventServiceBuilder {
	b.topic = value
	return b
}

// Build uses the data stored in the builder to create a new event service.
func (b *EventServiceBuilder) Build(ctx context.Context) (result *EventService, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.connection == nil {
		err = errors.New("connection is mandatory")
		return
	}
	if len(b.brokers) == 0 {
		err = errors.New("at least one broker is mandatory")
		return
	}
	if b.topic == "" {
		err = errors.New("topic is mandatory")
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
	kafkaBrokers := make([]string, len(b.brokers))
	copy(kafkaBrokers, b.brokers)

	// Create the bridges for the logging mechanism of the Kafka library:
	errorLogger := kafka.LoggerFunc(func(msg string, args ...interface{}) {
		logger.Error(ctx, msg, args...)
	})

	// Get the clients for the services that we will be using:
	accessReview := b.connection.Authorizations().V1().AccessReview()

	// Create the obfuscation cipher and make sure it has the block size that we expect:
	offsetCipher, err := aes.NewCipher(offsetEncodingKey)
	if err != nil {
		return
	}
	offsetCipherSize := offsetCipher.BlockSize()
	if offsetCipher.BlockSize() != offsetEncodingSize {
		err = fmt.Errorf(
			"block size of cipher used for encoding of offsets is %d, "+
				"but expected %d",
			offsetCipherSize, offsetEncodingSize,
		)
		return
	}

	// Create the data digger:
	dataDigger, err := data.NewDigger().Build(ctx)
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &EventService{
		logger:       b.logger,
		errorLogger:  errorLogger,
		accessReview: accessReview,
		brokers:      kafkaBrokers,
		topic:        b.topic,
		offsetCipher: offsetCipher,
		dataDigger:   dataDigger,
		dialer:       dialer,
	}

	return
}

func (b *EventServiceBuilder) createDialer(ctx context.Context) (dialer *kafka.Dialer, err error) {
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

func (b *EventServiceBuilder) createTopics(ctx context.Context, dialer *kafka.Dialer) error {
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
			Topic:             b.topic,
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

// EventListOptions contains the parameters for the List method.
type EventListOptions struct {
	// From is the identifier of the first event requested.
	From string

	// Callback is a function that will be called to process the events. When this function
	// returns an error the List method will finish and return that same error.
	Callback func(*models.Event) error
}

// List returns a retrieves a list of events and processes them with the callback function specified
// in the options.
func (s *EventService) List(ctx context.Context, options EventListOptions) error {
	var err error

	// Check parameters:
	if options.Callback == nil {
		return errors.New("callback is mandatory")
	}

	// Check that the from parameter is valid:
	offset := kafka.LastOffset
	if options.From != "" {
		offset, err = s.decodeOffset(options.From)
		if err != nil {
			return fmt.Errorf("value '%s' isn't a valid event identifier", options.From)
		}
	}

	// Create the message reader:
	reader := kafka.NewReader(kafka.ReaderConfig{
		Dialer:      s.dialer,
		ErrorLogger: s.errorLogger,
		Brokers:     s.brokers,
		Topic:       s.topic,
		Partition:   0,
	})
	defer func() {
		err := reader.Close()
		if err != nil {
			s.logger.Error(ctx, "Can't close reader: %v", err)
		}
	}()
	err = reader.SetOffset(offset)
	if err != nil {
		return err
	}
	for {
		message, err := reader.ReadMessage(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if s.logger.DebugEnabled() {
			s.logger.Debug(
				ctx,
				"Received message from topic '%s' with offset %d, "+
					"value follows:\n%s",
				message.Topic, message.Offset, string(message.Value),
			)
		}
		err = s.processMessage(ctx, message, options.Callback)
		if err != nil {
			return err
		}
	}
	return nil
}

// processMessage processes a message received from Kafka, checks if the user has permission to see
// it, and delivers it.
func (s *EventService) processMessage(ctx context.Context, message kafka.Message,
	callback func(*models.Event) error) error {
	// Parse the event:
	event := &models.Event{}
	err := json.Unmarshal(message.Value, event)
	if err != nil {
		return err
	}

	// Set the obfuscated offset as the identifier of the event now, so that it will be easily
	// available for logging while checking permission:
	event.ID, err = s.encodeOffset(message.Offset)
	if err != nil {
		return err
	}
	s.logger.Info(ctx, "Identifier for offset %d is '%s'", message.Offset, event.ID)

	// Check if the user has permission to see the event:
	ok, err := s.checkPermission(ctx, event)
	if err != nil {
		return err
	}
	if !ok {
		s.logger.Info(
			ctx,
			"User doesn't have permission to see event '%s' will skip it",
			event.ID,
		)
		return nil
	}

	// Call the function that does the actual processing of the event:
	err = callback(event)
	if err != nil {
		return err
	}

	return nil
}

// checkPermissions checks if the current user has permission to see the given event.
func (s *EventService) checkPermission(ctx context.Context, event *models.Event) (allowed bool,
	err error) {
	// Get the kind of the details:
	value := s.dataDigger.Dig(event.Details, "kind")
	if value == nil {
		s.logger.Info(
			ctx,
			"Can't get 'kind' field in order to determine how to check permission "+
				"for event '%s', will deny access",
			event.ID,
		)
		allowed = false
		err = nil
		return
	}
	kind, ok := value.(string)
	if !ok {
		s.logger.Info(
			ctx,
			"Expected 'kind' for event '%s' to be a string, but it is of type '%T', "+
				"will deny access",
			event.ID, value,
		)
		return
	}

	// Check the permission according to the type of the object contained in the details:
	switch kind {
	case "Change":
		allowed, err = s.checkChangePermission(ctx, event)
	default:
		s.logger.Info(
			ctx,
			"Don't know how to check permissions for details of kind '%s' in "+
				"event '%s', will deny access",
			event.ID,
			kind,
		)
	}
	return
}

// checkChangePermissions checks if the current user has permission to see the given change details.
func (s *EventService) checkChangePermission(ctx context.Context, event *models.Event) (allowed bool,
	err error) {
	// Get the user name from the context:
	user, err := s.getUser(ctx)
	if err != nil {
		return
	}

	// We will use this to extract values from the event details:
	var value interface{}

	// Figure out the resource type:
	value = s.dataDigger.Dig(event.Details, "object.kind")
	if value == nil {
		s.logger.Info(
			ctx,
			"Can't get 'object.kind' field in order to determine the resource type "+
				"for event '%s', will deny access",
			event.ID,
		)
		return
	}
	resource, ok := value.(string)
	if !ok {
		s.logger.Info(
			ctx,
			"Expected 'object.kind' for event '%s' to be a string, but it is of "+
				"type '%T', will deny access",
			event.ID, value,
		)
		return
	}

	// Figure out the resource identifier:
	value = s.dataDigger.Dig(event.Details, "object.id")
	if value == nil {
		s.logger.Info(
			ctx,
			"Can't get 'object.id' field in order to determine the object identifier "+
				"for event '%s', will deny access",
			event.ID,
		)
		return
	}
	id, ok := value.(string)
	if !ok {
		s.logger.Info(
			ctx,
			"Expected 'object.id' for event '%s' to be a string, but it is of "+
				"type '%T', will deny access",
			event.ID, value,
		)
		return
	}

	// Prepare the access review request:
	builder := azv1.NewAccessReviewRequest()
	builder.AccountUsername(user)
	builder.Action("get")
	switch resource {
	case cmv1.ClusterKind, cmv1.ClusterLinkKind:
		builder.ResourceType("Cluster")
		builder.ClusterID(id)
	case amv1.SubscriptionKind, amv1.SubscriptionLinkKind:
		builder.ResourceType("Subscription")
		builder.SubscriptionID(id)
	default:
		s.logger.Info(
			ctx,
			"Don't know how to populate access review request for event '%s' and "+
				"resource type '%s' will deny access",
			event.ID, resource,
		)
		return
	}

	// Send the access review request:
	s.logger.Debug(
		ctx,
		"Sending access review request for event '%s', user '%s', resource type '%s' "+
			"and object identifier '%s'",
		event.ID, user, resource, id,
	)
	request, err := builder.Build()
	if err != nil {
		return
	}
	response, err := s.accessReview.Post().
		Request(request).
		SendContext(ctx)
	if err != nil {
		return
	}

	// Return the result:
	allowed = response.Response().Allowed()
	s.logger.Debug(
		ctx,
		"Result of access review request for event '%s', user '%s', resource type '%s' "+
			"and object identifier '%s' is '%v'",
		event.ID, user, resource, id, allowed,
	)
	return
}

// getUser extracts the user name from the context.
func (s *EventService) getUser(ctx context.Context) (user string, err error) {
	token, err := authentication.TokenFromContext(ctx)
	if err != nil {
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		err = fmt.Errorf("type of claims is '%T', but expected a map", token.Claims)
		return
	}
	claim, ok := claims["username"]
	if !ok {
		err = fmt.Errorf("expected 'user' claim, but it is missing")
		return
	}
	user, ok = claim.(string)
	if !ok {
		err = fmt.Errorf("type of 'user' claim is '%T', but expected string", claim)
		return
	}
	return
}

// encodeOffset encodes and obfuscates the given Kafka offset for use as an event identifier. The
// intent is to ensure that it doesn't look like a sequential number, should make it less likely
// incorrect use and gives us more freedom to change the format and content of event identifiers in
// the future.
func (s *EventService) encodeOffset(offset int64) (result string, err error) {
	buffer := &bytes.Buffer{}
	buffer.Grow(offsetEncodingSize)
	err = binary.Write(buffer, binary.BigEndian, offsetEncodingMagic)
	if err != nil {
		return
	}
	err = binary.Write(buffer, binary.BigEndian, offset)
	if err != nil {
		return
	}
	plaintext := buffer.Bytes()
	encrypted := make([]byte, offsetEncodingSize)
	s.offsetCipher.Encrypt(encrypted, plaintext)
	result = offsetEncoding.EncodeToString(encrypted)
	return
}

// decodeOffset extracts the Kafka offset from the given encoded and obfuscated event identifier.
func (s *EventService) decodeOffset(id string) (offset int64, err error) {
	encrypted, err := offsetEncoding.DecodeString(id)
	if err != nil {
		return
	}
	if len(encrypted) != offsetEncodingSize {
		err = fmt.Errorf(
			"encrypted text is %d bytes long, but expected %d",
			len(encrypted), offsetEncodingSize,
		)
		return
	}
	plaintext := make([]byte, offsetEncodingSize)
	s.offsetCipher.Decrypt(plaintext, encrypted)
	buffer := bytes.NewBuffer(plaintext)
	var magic int64
	err = binary.Read(buffer, binary.BigEndian, &magic)
	if err != nil {
		return
	}
	if magic != offsetEncodingMagic {
		err = fmt.Errorf(
			"magic number is %d, but expected %d",
			magic, offsetEncodingMagic,
		)
		return
	}
	err = binary.Read(buffer, binary.BigEndian, &offset)
	return
}

// Close releases all the resources used by the service.
func (s *EventService) Close() error {
	return nil
}

// offsetEncodingKey is an AES 128 bits key that is used to encrypt the offsets. Note that the
// intention here is to obfuscate Kafka offsets, not to keep them private. This key can be
// considered public.
var offsetEncodingKey = []byte{
	0x36, 0x2b, 0x0a, 0x4b, 0x7c, 0x82, 0x72, 0x02,
	0xa6, 0xb2, 0xf6, 0x44, 0x18, 0x64, 0xa6, 0x34,
}

// offsetEncodingSize is the expected block size of the cipher used for obfuscation of Kafka
// offsets.
const offsetEncodingSize = 16

// offsetEncodingMagic is the magic number that is combined with offsets to generate the data that
// will then be encrypted to obfuscate them. It is used to validate that received identifiers are
// correct.
const offsetEncodingMagic = int64(0x7df934cd66a9673b)

// offsetEncodingAlphabet is the lower case alphabet used to encode obfuscated offsets.
const offsetEncodingAlphabet = "0123456789abcdefghijklmnopqrstuv"

// offsetEncoding is the lower case variant of Base32 used to encode obfuscated offsets.
var offsetEncoding = base32.NewEncoding(offsetEncodingAlphabet).WithPadding(base32.NoPadding)
