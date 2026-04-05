package conduitclient

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MessageType represents the type of message
type MessageType string

const (
	// MessageTypeData is a regular data message
	MessageTypeData MessageType = "data"

	// MessageTypeControl is a control plane message
	MessageTypeControl MessageType = "control"

	// MessageTypeError is an error message
	MessageTypeError MessageType = "error"
)

// Message represents a message in the Exchange
type Message struct {
	// ID is a unique identifier for this message
	ID string `json:"id"`

	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`

	// Sequence is the NATS stream sequence number
	Sequence uint64 `json:"sequence"`

	// Type is the message type
	Type MessageType `json:"type"`

	// Payload is the message content (arbitrary JSON)
	Payload json.RawMessage `json:"payload"`

	// Metadata contains optional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UnmarshalPayload unmarshals the payload into the given struct
func (m *Message) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// NewMessage creates a new message with the given payload
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Type:      msgType,
		Payload:   payloadBytes,
		Metadata:  make(map[string]string),
	}, nil
}

// NewDataMessage creates a new data message (convenience function)
func NewDataMessage(payload interface{}) (*Message, error) {
	return NewMessage(MessageTypeData, payload)
}

// MarshalMessage marshals a message to JSON bytes
func MarshalMessage(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalMessage unmarshals a message from JSON bytes
func UnmarshalMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	// Validate the message
	if err := ValidateMessage(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// ValidateMessage validates that a message has all required fields
func ValidateMessage(msg *Message) error {
	if msg.ID == "" {
		return fmt.Errorf("message ID is required")
	}
	if msg.Type == "" {
		return fmt.Errorf("message type is required")
	}
	if msg.Type != MessageTypeData && msg.Type != MessageTypeControl && msg.Type != MessageTypeError {
		return fmt.Errorf("invalid message type: %s", msg.Type)
	}
	if len(msg.Payload) == 0 {
		return fmt.Errorf("message payload is required")
	}
	return nil
}
