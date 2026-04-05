package conduitclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

// ResponseHandler is a function that processes responses from an Exchange
type ResponseHandler func(*Message) error

// ExchangeClient is a client for interacting with Conduit Exchange workers
type ExchangeClient struct {
	nc            *nats.Conn
	js            nats.JetStreamContext
	subjectInput  string
	subjectOutput string

	// Public fields for Conduit to reference
	Name      string
	Namespace string
}

// Conduit manages Exchange lifecycle via the API
type Conduit struct {
	apiBaseURL string
	apiKey     string
	httpClient *http.Client
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ExchangeRequest represents a request to create an Exchange via API
type ExchangeRequest struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace,omitempty"`
	Image     string   `json:"image,omitempty"`
	Env       []EnvVar `json:"env,omitempty"`
}

// ExchangeResource represents an Exchange from the API
type ExchangeResource struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	UID            string         `json:"uid"`
	Phase          string         `json:"phase"`
	Message        string         `json:"message,omitempty"`
	ConnectionInfo ConnectionInfo `json:"connectionInfo"`
}

// ConnectionInfo provides NATS connection details
type ConnectionInfo struct {
	NATSURL       string `json:"natsURL"`
	StreamName    string `json:"streamName"`
	InputSubject  string `json:"inputSubject"`
	OutputSubject string `json:"outputSubject"`
}

// NewConduit creates a new Conduit API client
func NewConduit(apiBaseURL string, apiKey string) *Conduit {
	return &Conduit{
		apiBaseURL: apiBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateExchangeClient creates a new Exchange via the API and returns a connected client
func (c *Conduit) CreateExchangeClient(req ExchangeRequest) (*ExchangeClient, error) {
	// Create the Exchange
	resource, err := c.createExchange(req)
	if err != nil {
		return nil, err
	}

	// Wait for it to be ready
	resource, err = c.waitForExchangeReady(resource.Name, resource.Namespace, 60*time.Second)
	if err != nil {
		return nil, err
	}

	// Connect to NATS
	nc, err := nats.Connect(resource.ConnectionInfo.NATSURL,
		nats.MaxReconnects(-1),
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

	return &ExchangeClient{
		nc:            nc,
		js:            js,
		subjectInput:  resource.ConnectionInfo.InputSubject,
		subjectOutput: resource.ConnectionInfo.OutputSubject,
		Name:          resource.Name,
		Namespace:     resource.Namespace,
	}, nil
}

// DeleteExchangeClient deletes an Exchange via the API
func (c *Conduit) DeleteExchangeClient(exchange *ExchangeClient) error {
	return c.deleteExchange(exchange.Name, exchange.Namespace)
}

// Send sends a message to the Exchange without waiting for a response
func (c *ExchangeClient) Send(ctx context.Context, payload interface{}) error {
	msg, err := NewDataMessage(payload)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = c.js.Publish(c.subjectInput, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Request sends a message and waits for a single response with a timeout
// This uses a temporary inbox subscription for the response
func (c *ExchangeClient) Request(ctx context.Context, payload interface{}, timeout time.Duration) (*Message, error) {
	msg, err := NewDataMessage(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	data, err := MarshalMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a temporary subscription for the response
	// We'll correlate by message ID
	responseChan := make(chan *Message, 1)
	errChan := make(chan error, 1)

	// Subscribe to output subject
	sub, err := c.nc.Subscribe(c.subjectOutput, func(natsMsg *nats.Msg) {
		respMsg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			errChan <- err
			return
		}

		// Check if this response correlates to our request
		// Note: This is a simple implementation. For production, you might want
		// to add correlation IDs in metadata
		responseChan <- respMsg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to output: %w", err)
	}
	defer sub.Unsubscribe()

	// Publish the request
	_, err = c.js.Publish(c.subjectInput, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for response or timeout
	select {
	case resp := <-responseChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout after %v", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Subscribe subscribes to all responses from the Exchange
// The handler will be called for each response message
func (c *ExchangeClient) Subscribe(ctx context.Context, handler ResponseHandler) error {
	// Subscribe to output subject
	sub, err := c.nc.Subscribe(c.subjectOutput, func(natsMsg *nats.Msg) {
		msg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			// Log error but continue processing
			return
		}

		// Call handler
		_ = handler(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe
	sub.Unsubscribe()

	return ctx.Err()
}

// SubscribeWithQueue subscribes to responses using a queue group
// This allows multiple clients to load-balance response processing
func (c *ExchangeClient) SubscribeWithQueue(ctx context.Context, queueGroup string, handler ResponseHandler) error {
	// Subscribe to output subject with queue group
	sub, err := c.nc.QueueSubscribe(c.subjectOutput, queueGroup, func(natsMsg *nats.Msg) {
		msg, err := UnmarshalMessage(natsMsg.Data)
		if err != nil {
			return
		}

		_ = handler(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe
	sub.Unsubscribe()

	return ctx.Err()
}

// Close closes the NATS connection
func (c *ExchangeClient) Close() error {
	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

// Private API helper methods for Conduit

func (c *Conduit) createExchange(req ExchangeRequest) (*ExchangeResource, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.apiBaseURL+"/api/v1/exchanges", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var resource ExchangeResource
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resource, nil
}

func (c *Conduit) getExchange(name, namespace string) (*ExchangeResource, error) {
	url := fmt.Sprintf("%s/api/v1/exchanges/%s/%s", c.apiBaseURL, namespace, name)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var resource ExchangeResource
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resource, nil
}

func (c *Conduit) waitForExchangeReady(name, namespace string, timeout time.Duration) (*ExchangeResource, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resource, err := c.getExchange(name, namespace)
		if err != nil {
			return nil, err
		}

		if resource.Phase == "Running" {
			return resource, nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("Exchange %s/%s did not become ready within %v", namespace, name, timeout)
}

func (c *Conduit) deleteExchange(name, namespace string) error {
	url := fmt.Sprintf("%s/api/v1/exchanges/%s/%s", c.apiBaseURL, namespace, name)

	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
