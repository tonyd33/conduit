package conduit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// MessageHandler is a function that processes incoming messages
// Return nil to acknowledge the message, or an error to nack it
type MessageHandler func(context.Context, *Message) error

// Client is the main Conduit SDK client
type Client struct {
	config *Config

	// NATS connection
	nc   *nats.Conn
	js   nats.JetStreamContext
	sub  *nats.Subscription

	// Control
	paused bool
	mu     sync.RWMutex
}

// NewClient creates a new Conduit client
// It automatically loads configuration from environment variables
func NewClient() (*Client, error) {
	return NewClientWithConfig(nil)
}

// NewClientWithConfig creates a new client with custom configuration
// If config is nil, it loads from environment variables
func NewClientWithConfig(config *Config) (*Client, error) {
	if config == nil {
		var err error
		config, err = LoadConfigFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Connect to NATS
	nc, err := nats.Connect(config.NATSURL,
		nats.Name(fmt.Sprintf("conduit-exchange-%s", config.ExchangeID)),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &Client{
		config: config,
		nc:     nc,
		js:     js,
	}

	return client, nil
}

// Run starts processing messages with the given handler
// This function blocks until the context is cancelled or an error occurs
func (c *Client) Run(ctx context.Context, handler MessageHandler) error {
	// Bind to the existing pull consumer
	sub, err := c.js.PullSubscribe(
		c.config.SubjectInput,
		c.config.ConsumerName,
		nats.Bind(c.config.StreamName, c.config.ConsumerName),
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	c.sub = sub

	// Process messages in a loop
	go c.processMessages(ctx, sub, handler)

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe and drain
	_ = sub.Unsubscribe()

	return ctx.Err()
}

// processMessages continuously pulls and processes messages
func (c *Client) processMessages(ctx context.Context, sub *nats.Subscription, handler MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Fetch messages (batch of 1, timeout 1 second)
			msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					// No messages available, continue
					continue
				}
				// For other errors, continue (will retry)
				continue
			}

			// Process each message
			for _, natsMsg := range msgs {
				c.handleMessage(ctx, natsMsg, handler)
			}
		}
	}
}

// handleMessage processes a single message
func (c *Client) handleMessage(ctx context.Context, natsMsg *nats.Msg, handler MessageHandler) {
	// Check if paused
	c.mu.RLock()
	if c.paused {
		c.mu.RUnlock()
		// Nack and let it be redelivered later
		_ = natsMsg.Nak()
		return
	}
	c.mu.RUnlock()

	// Parse message
	msg, err := UnmarshalMessage(natsMsg.Data)
	if err != nil {
		_ = natsMsg.Term() // Terminal error, don't retry
		return
	}

	// Get sequence from metadata
	meta, err := natsMsg.Metadata()
	if err == nil {
		msg.Sequence = meta.Sequence.Stream
	}

	// Handle control messages
	if msg.Type == MessageTypeControl {
		c.handleControlMessage(msg)
		_ = natsMsg.Ack()
		return
	}

	// Call user handler
	if err := handler(ctx, msg); err != nil {
		_ = natsMsg.Nak()
		return
	}

	// Ack message
	_ = natsMsg.Ack()
}


// handleControlMessage processes control messages
func (c *Client) handleControlMessage(msg *Message) {
	var ctrl ControlMessage
	if err := msg.UnmarshalPayload(&ctrl); err != nil {
		return
	}

	switch ctrl.Command {
	case ControlCommandPause:
		c.mu.Lock()
		c.paused = true
		c.mu.Unlock()

	case ControlCommandResume:
		c.mu.Lock()
		c.paused = false
		c.mu.Unlock()

	case ControlCommandShutdown:
		// TODO: Trigger graceful shutdown
	}
}

// Publish publishes a message to the output subject
func (c *Client) Publish(ctx context.Context, payload interface{}) error {
	msg, err := NewMessage(MessageTypeData, payload)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = c.js.Publish(c.config.SubjectOutput, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Close closes the client and cleans up resources
func (c *Client) Close() error {
	if c.sub != nil {
		_ = c.sub.Unsubscribe()
	}

	if c.nc != nil {
		c.nc.Close()
	}

	return nil
}
