package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
)

// StreamManager handles NATS JetStream stream operations
type StreamManager struct {
	client *Client
}

// NewStreamManager creates a new StreamManager
func NewStreamManager(client *Client) *StreamManager {
	return &StreamManager{
		client: client,
	}
}

// StreamConfig represents the configuration for a NATS stream
type StreamConfig struct {
	Name     string
	Subjects []string
	MaxAge   time.Duration
	MaxMsgs  int64
	MaxBytes int64
	Replicas int
}

// CreateOrUpdateStream creates or updates a JetStream stream for an Exchange
func (sm *StreamManager) CreateOrUpdateStream(streamName string, exchange *conduitv1alpha1.Exchange) error {
	// Build subject patterns
	exchangeID := string(exchange.UID)
	subjects := []string{
		fmt.Sprintf("exchange.%s.input", exchangeID),
		fmt.Sprintf("exchange.%s.output", exchangeID),
		fmt.Sprintf("exchange.%s.control", exchangeID),
	}

	// Use custom subjects if specified
	if exchange.Spec.Stream.Subjects.Input != "" {
		subjects[0] = exchange.Spec.Stream.Subjects.Input
	}
	if exchange.Spec.Stream.Subjects.Output != "" {
		subjects[1] = exchange.Spec.Stream.Subjects.Output
	}
	if exchange.Spec.Stream.Subjects.Control != "" {
		subjects[2] = exchange.Spec.Stream.Subjects.Control
	}

	// Parse retention configuration
	maxAge := 7 * 24 * time.Hour // Default: 7 days
	if exchange.Spec.Stream.Retention.MaxAge.Duration > 0 {
		maxAge = exchange.Spec.Stream.Retention.MaxAge.Duration
	}

	maxMsgs := int64(100000) // Default: 100k messages
	if exchange.Spec.Stream.Retention.MaxMessages > 0 {
		maxMsgs = exchange.Spec.Stream.Retention.MaxMessages
	}

	maxBytes := int64(1 << 30) // Default: 1GB
	if exchange.Spec.Stream.Retention.MaxBytes != nil {
		maxBytes = exchange.Spec.Stream.Retention.MaxBytes.Value()
	}

	// Create stream configuration
	cfg := &nats.StreamConfig{
		Name:       streamName,
		Subjects:   subjects,
		Retention:  nats.LimitsPolicy,
		MaxAge:     maxAge,
		MaxMsgs:    maxMsgs,
		MaxBytes:   maxBytes,
		Storage:    nats.FileStorage,
		Replicas:   1, // TODO: Make configurable for HA
		Discard:    nats.DiscardOld,
		NoAck:      false, // Require explicit acks
		Duplicates: 1 * time.Minute,
	}

	// Try to get existing stream
	js := sm.client.JetStream()
	streamInfo, err := js.StreamInfo(streamName)

	if err != nil {
		// Stream doesn't exist, create it
		if err == nats.ErrStreamNotFound {
			_, err = js.AddStream(cfg)
			if err != nil {
				return fmt.Errorf("failed to create stream %s: %w", streamName, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get stream info for %s: %w", streamName, err)
	}

	// Stream exists, update if needed
	if !streamsEqual(&streamInfo.Config, cfg) {
		_, err = js.UpdateStream(cfg)
		if err != nil {
			return fmt.Errorf("failed to update stream %s: %w", streamName, err)
		}
	}

	return nil
}

// DeleteStream deletes a JetStream stream
func (sm *StreamManager) DeleteStream(streamName string) error {
	js := sm.client.JetStream()
	err := js.DeleteStream(streamName)
	if err != nil && err != nats.ErrStreamNotFound {
		return fmt.Errorf("failed to delete stream %s: %w", streamName, err)
	}
	return nil
}

// StreamExists checks if a stream exists
func (sm *StreamManager) StreamExists(streamName string) (bool, error) {
	js := sm.client.JetStream()
	_, err := js.StreamInfo(streamName)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check stream existence: %w", err)
	}
	return true, nil
}

// GetStreamInfo returns information about a stream
func (sm *StreamManager) GetStreamInfo(streamName string) (*nats.StreamInfo, error) {
	js := sm.client.JetStream()
	info, err := js.StreamInfo(streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}
	return info, nil
}

// CreateConsumer creates a durable consumer for a stream
func (sm *StreamManager) CreateConsumer(streamName, consumerName string, subjects []string) error {
	js := sm.client.JetStream()

	// Filter subject - if multiple, use wildcard or create multiple consumers
	filterSubject := ""
	if len(subjects) > 0 {
		filterSubject = subjects[0] // For now, filter on input subject
	}

	cfg := &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		FilterSubject: filterSubject,
		ReplayPolicy:  nats.ReplayInstantPolicy,
	}

	_, err := js.AddConsumer(streamName, cfg)
	if err != nil {
		// Check if consumer already exists
		if err == nats.ErrConsumerNameAlreadyInUse {
			return nil // Consumer already exists, that's fine
		}
		return fmt.Errorf("failed to create consumer %s: %w", consumerName, err)
	}

	return nil
}

// DeleteConsumer deletes a consumer
func (sm *StreamManager) DeleteConsumer(streamName, consumerName string) error {
	js := sm.client.JetStream()
	err := js.DeleteConsumer(streamName, consumerName)
	if err != nil && err != nats.ErrConsumerNotFound {
		return fmt.Errorf("failed to delete consumer %s: %w", consumerName, err)
	}
	return nil
}

// GetConsumerInfo returns information about a consumer
func (sm *StreamManager) GetConsumerInfo(streamName, consumerName string) (*nats.ConsumerInfo, error) {
	js := sm.client.JetStream()
	info, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer info: %w", err)
	}
	return info, nil
}

// streamsEqual compares two stream configs (simplified comparison)
func streamsEqual(a, b *nats.StreamConfig) bool {
	if a.Name != b.Name {
		return false
	}
	if a.MaxAge != b.MaxAge {
		return false
	}
	if a.MaxMsgs != b.MaxMsgs {
		return false
	}
	if a.MaxBytes != b.MaxBytes {
		return false
	}
	// Note: Not comparing subjects as they should be immutable
	return true
}
