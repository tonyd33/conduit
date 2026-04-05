package nats

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

// Client wraps a NATS connection and provides JetStream management
type Client struct {
	conn *nats.Conn
	js   nats.JetStreamContext
	url  string
}

// ClientConfig holds configuration for creating a NATS client
type ClientConfig struct {
	// URL is the NATS server URL (e.g., "nats://nats.default.svc:4222")
	URL string

	// ConnectionTimeout is the timeout for connecting to NATS
	ConnectionTimeout time.Duration

	// MaxReconnects is the maximum number of reconnection attempts
	MaxReconnects int

	// ReconnectWait is the time to wait between reconnection attempts
	ReconnectWait time.Duration

	// Authentication options (mutually exclusive)

	// Username for basic authentication
	Username string
	// Password for basic authentication
	Password string

	// NKeyFile path to NKey seed file
	NKeyFile string

	// CredentialsFile path to JWT credentials file
	CredentialsFile string

	// Token for token-based authentication
	Token string

	// TLS options

	// TLSCertFile path to TLS client certificate
	TLSCertFile string
	// TLSKeyFile path to TLS client key
	TLSKeyFile string
	// TLSCAFile path to TLS CA certificate for server verification
	TLSCAFile string
}

// DefaultConfig returns a default NATS client configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		URL:               "nats://nats.default.svc.cluster.local:4222",
		ConnectionTimeout: 10 * time.Second,
		MaxReconnects:     5,
		ReconnectWait:     2 * time.Second,
	}
}

// NewClient creates a new NATS client with the given configuration
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// NATS connection options
	opts := []nats.Option{
		nats.Name("conduit-operator"),
		nats.Timeout(config.ConnectionTimeout),
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectWait(config.ReconnectWait),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				fmt.Printf("NATS disconnected: %v\n", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
	}

	// Add authentication options
	if err := addAuthOptions(config, &opts); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Add TLS options
	if err := addTLSOptions(config, &opts); err != nil {
		return nil, fmt.Errorf("failed to configure TLS: %w", err)
	}

	// Connect to NATS
	conn, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", config.URL, err)
	}

	// Create JetStream context
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Client{
		conn: conn,
		js:   js,
		url:  config.URL,
	}, nil
}

// addAuthOptions adds authentication options to NATS connection
func addAuthOptions(config *ClientConfig, opts *[]nats.Option) error {
	// Username/Password authentication
	if config.Username != "" && config.Password != "" {
		*opts = append(*opts, nats.UserInfo(config.Username, config.Password))
		return nil
	}

	// Token authentication
	if config.Token != "" {
		*opts = append(*opts, nats.Token(config.Token))
		return nil
	}

	// NKey authentication
	if config.NKeyFile != "" {
		opt, err := nats.NkeyOptionFromSeed(config.NKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load NKey from %s: %w", config.NKeyFile, err)
		}
		*opts = append(*opts, opt)
		return nil
	}

	// JWT/Credentials file authentication
	if config.CredentialsFile != "" {
		*opts = append(*opts, nats.UserCredentials(config.CredentialsFile))
		return nil
	}

	// No authentication configured
	return nil
}

// addTLSOptions adds TLS options to NATS connection
func addTLSOptions(config *ClientConfig, opts *[]nats.Option) error {
	// If no TLS files specified, return early
	if config.TLSCertFile == "" && config.TLSKeyFile == "" && config.TLSCAFile == "" {
		return nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load client certificate and key
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for server verification
	if config.TLSCAFile != "" {
		caCert, err := os.ReadFile(config.TLSCAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	*opts = append(*opts, nats.Secure(tlsConfig))
	return nil
}

// Close closes the NATS connection
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

// IsConnected returns true if the client is connected to NATS
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

// JetStream returns the JetStream context
func (c *Client) JetStream() nats.JetStreamContext {
	return c.js
}

// Connection returns the underlying NATS connection
func (c *Client) Connection() *nats.Conn {
	return c.conn
}

// URL returns the NATS server URL
func (c *Client) URL() string {
	return c.url
}

// GetStreamInfo returns information about a stream
func (c *Client) GetStreamInfo(streamName string) (*nats.StreamInfo, error) {
	info, err := c.js.StreamInfo(streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}
	return info, nil
}

// GetConsumerInfo returns information about a consumer
func (c *Client) GetConsumerInfo(streamName, consumerName string) (*nats.ConsumerInfo, error) {
	info, err := c.js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer info: %w", err)
	}
	return info, nil
}
