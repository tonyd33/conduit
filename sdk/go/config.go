package conduit

import (
	"fmt"
	"os"
)

// Config holds the configuration for the Exchange client
type Config struct {
	// Exchange metadata
	ExchangeID        string
	ExchangeName      string
	ExchangeNamespace string

	// NATS configuration
	NATSURL         string
	StreamName      string
	ConsumerName    string
	SubjectInput    string
	SubjectOutput   string
	SubjectControl  string
}

// LoadConfigFromEnv loads configuration from environment variables
// These are injected by the Conduit operator into the pod
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{
		ExchangeID:        os.Getenv("EXCHANGE_ID"),
		ExchangeName:      os.Getenv("EXCHANGE_NAME"),
		ExchangeNamespace: os.Getenv("EXCHANGE_NAMESPACE"),
		NATSURL:           os.Getenv("NATS_URL"),
		StreamName:        os.Getenv("NATS_STREAM_NAME"),
		ConsumerName:      os.Getenv("NATS_CONSUMER_NAME"),
		SubjectInput:      os.Getenv("NATS_SUBJECT_INPUT"),
		SubjectOutput:     os.Getenv("NATS_SUBJECT_OUTPUT"),
		SubjectControl:    os.Getenv("NATS_SUBJECT_CONTROL"),
	}

	// Validate required fields
	if config.ExchangeID == "" {
		return nil, fmt.Errorf("EXCHANGE_ID environment variable is required")
	}
	if config.NATSURL == "" {
		return nil, fmt.Errorf("NATS_URL environment variable is required")
	}
	if config.StreamName == "" {
		return nil, fmt.Errorf("NATS_STREAM_NAME environment variable is required")
	}
	if config.ConsumerName == "" {
		return nil, fmt.Errorf("NATS_CONSUMER_NAME environment variable is required")
	}
	if config.SubjectInput == "" {
		return nil, fmt.Errorf("NATS_SUBJECT_INPUT environment variable is required")
	}

	return config, nil
}
