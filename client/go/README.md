# Conduit Go Client

A unified Go client library for managing Conduit Exchanges and communicating with Exchange workers.

## Features

- **Unified API**: Single client handles both Exchange lifecycle management and message communication
- **API Integration**: Create and manage Exchanges via the Conduit API server
- **NATS Messaging**: Send and receive messages using NATS JetStream
- **Automatic Setup**: Automatically retrieves connection info from API
- **Full Lifecycle**: Create, communicate, and clean up Exchanges with simple API

## Installation

```bash
go get github.com/tonyd33/conduit/client/go
```

## Quick Start: End-to-End with API

The simplest way to use Conduit - create an Exchange and start communicating:

```go
package main

import (
    "context"
    "fmt"
    conduit "github.com/tonyd33/conduit/client/go"
)

func main() {
    // Create Exchange and get a connected client in one step
    client, err := conduit.CreateExchange("http://localhost:8090", conduit.ExchangeRequest{
        Name:      "my-exchange",
        Namespace: "default",
        Image:     "my-worker-image:latest",
    }, "") // Empty string = no API key
    if err != nil {
        panic(err)
    }

    // Clean up when done
    defer func() {
        client.Close()                // Close NATS connection
        client.DeleteExchange()       // Delete Exchange via API
    }()

    // Send messages
    err = client.Send(context.Background(), "Hello, Exchange!")
    if err != nil {
        panic(err)
    }

    // Subscribe to responses
    ctx := context.Background()
    client.Subscribe(ctx, func(msg *conduit.Message) error {
        fmt.Println("Response:", string(msg.Payload))
        return nil
    })
}
```

## Usage Patterns

### Pattern 1: Full Lifecycle with API (Recommended)

Create Exchange via API, communicate, then clean up:

```go
// Create Exchange and connect
client, err := conduit.CreateExchange("http://localhost:8090", conduit.ExchangeRequest{
    Name:  "my-exchange",
    Image: "worker:latest",
}, "")

defer client.Close()
defer client.DeleteExchange()

// Use client for messaging...
```

### Pattern 2: Connect to Existing Exchange via API

Connect to an Exchange that already exists:

```go
client, err := conduit.NewExchangeClient(&conduit.ClientConfig{
    APIBaseURL:        "http://localhost:8090",
    ExchangeName:      "existing-exchange",
    ExchangeNamespace: "default",
})

// NATS URL and subjects are automatically fetched from API
```

### Pattern 3: Direct NATS Connection (No API)

Connect directly if you already know the NATS subjects:

```go
client, err := conduit.NewExchangeClient(&conduit.ClientConfig{
    NATSURL:       "nats://localhost:4222",
    SubjectInput:  "exchange.my-exchange.input",
    SubjectOutput: "exchange.my-exchange.output",
})
```

## Messaging Examples

### Simple Send (Fire and Forget)

```go
package main

import (
    "context"
    conduitclient "github.com/tonyd33/conduit/client/go"
)

func main() {
    client, err := conduitclient.NewExchangeClient(&conduitclient.ClientConfig{
        NATSURL:       "nats://localhost:4222",
        SubjectInput:  "exchange.my-exchange.input",
        SubjectOutput: "exchange.my-exchange.output",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Send a message without waiting for response
    err = client.Send(context.Background(), map[string]string{
        "message": "Hello, Exchange!",
    })
    if err != nil {
        panic(err)
    }
}
```

### Request-Response

```go
package main

import (
    "context"
    "fmt"
    "time"

    conduitclient "github.com/tonyd33/conduit/client/go"
)

func main() {
    client, err := conduitclient.NewExchangeClient(&conduitclient.ClientConfig{
        NATSURL:       "nats://localhost:4222",
        SubjectInput:  "exchange.my-exchange.input",
        SubjectOutput: "exchange.my-exchange.output",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Send a message and wait for response (with 5 second timeout)
    response, err := client.Request(context.Background(),
        map[string]string{"message": "Hello!"},
        5*time.Second,
    )
    if err != nil {
        panic(err)
    }

    // Parse the response
    var result map[string]interface{}
    response.UnmarshalPayload(&result)
    fmt.Println("Response:", result)
}
```

### Subscribe to All Responses

```go
package main

import (
    "context"
    "fmt"

    conduitclient "github.com/tonyd33/conduit/client/go"
)

func main() {
    client, err := conduitclient.NewExchangeClient(&conduitclient.ClientConfig{
        NATSURL:       "nats://localhost:4222",
        SubjectInput:  "exchange.my-exchange.input",
        SubjectOutput: "exchange.my-exchange.output",
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Subscribe to all responses
    go client.Subscribe(ctx, func(msg *conduitclient.Message) error {
        var payload map[string]interface{}
        msg.UnmarshalPayload(&payload)
        fmt.Println("Received response:", payload)
        return nil
    })

    // Send some messages
    for i := 0; i < 10; i++ {
        client.Send(context.Background(), map[string]int{"count": i})
    }

    // Keep running
    select {}
}
```

## Finding Exchange Subjects

### Option 1: Use Custom Subjects

Define custom subjects in your Exchange spec:

```yaml
apiVersion: conduit.mnke.org/v1alpha1
kind: Exchange
metadata:
  name: my-exchange
spec:
  stream:
    subjects:
      input: "my-app.input"
      output: "my-app.output"
```

### Option 2: Use Default UID-based Subjects

Get the Exchange UID and construct subjects:

```bash
# Get Exchange UID
kubectl get exchange my-exchange -o jsonpath='{.metadata.uid}'

# Subjects will be:
# - Input:  exchange.{uid}.input
# - Output: exchange.{uid}.output
```

## API Reference

### Exchange Lifecycle

#### `CreateExchange(apiBaseURL string, req ExchangeRequest, apiKey string) (*ExchangeClient, error)`

Creates a new Exchange via the API and returns a fully connected client. This is the easiest way to get started.

**Parameters:**
- `apiBaseURL`: Conduit API server URL (e.g., "http://localhost:8090")
- `req`: Exchange configuration (name, namespace, image, env vars, etc.)
- `apiKey`: Optional API key for authentication (empty string if not needed)

**Returns:** A connected `ExchangeClient` ready to send/receive messages

**Example:**
```go
client, err := conduit.CreateExchange("http://localhost:8090", conduit.ExchangeRequest{
    Name:      "my-exchange",
    Namespace: "default",
    Image:     "worker:latest",
    Env: map[string]string{
        "KEY": "value",
    },
}, "")
```

#### `NewExchangeClient(config *ClientConfig) (*ExchangeClient, error)`

Creates a new client for interacting with an Exchange. Supports three modes:
1. **API mode**: Set `APIBaseURL` and `ExchangeName` to auto-fetch connection info
2. **Direct mode**: Set `NATSURL` and subjects for direct connection
3. **Hybrid mode**: Combine API and custom configuration

**Example (API mode):**
```go
client, err := conduit.NewExchangeClient(&conduit.ClientConfig{
    APIBaseURL:        "http://localhost:8090",
    ExchangeName:      "existing-exchange",
    ExchangeNamespace: "default",
})
```

**Example (Direct mode):**
```go
client, err := conduit.NewExchangeClient(&conduit.ClientConfig{
    NATSURL:       "nats://localhost:4222",
    SubjectInput:  "exchange.abc-123.input",
    SubjectOutput: "exchange.abc-123.output",
})
```

#### `DeleteExchange() error`

Deletes the Exchange via the API (requires API configuration).

### Messaging

#### `Send(ctx context.Context, payload interface{}) error`

Sends a message to the Exchange without waiting for a response.

#### `Request(ctx context.Context, payload interface{}, timeout time.Duration) (*Message, error)`

Sends a message and waits for a single response with a timeout.

#### `Subscribe(ctx context.Context, handler ResponseHandler) error`

Subscribes to all responses from the Exchange. Blocks until context is cancelled.

#### `SubscribeWithQueue(ctx context.Context, queueGroup string, handler ResponseHandler) error`

Subscribes to responses using a NATS queue group for load balancing.

### Connection

#### `Close() error`

Closes the NATS connection.

## Message Format

All messages use the Conduit envelope format:

```json
{
  "id": "uuid",
  "timestamp": "2024-01-01T00:00:00Z",
  "sequence": 0,
  "type": "data",
  "payload": {
    "your": "data here"
  },
  "metadata": {
    "optional": "metadata"
  }
}
```

The client library automatically wraps your payload in this envelope format.
