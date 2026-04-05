# Conduit Go SDK

The Conduit Go SDK provides a simple, idiomatic interface for building Exchange-compatible applications in Go. It handles all the complexity of NATS JetStream integration, checkpoint management, and graceful lifecycle handling.

## Features

- **Automatic Configuration**: Loads all necessary configuration from environment variables injected by the Conduit operator
- **Message Handling**: Type-safe message envelope with automatic marshaling/unmarshaling
- **State Management**: Built-in support for application state with checkpointing
- **Recovery**: Automatic recovery from the last checkpoint when pods restart
- **Graceful Shutdown**: Proper cleanup of NATS connections and resources
- **Control Messages**: Handle pause/resume commands from the operator

## Installation

```bash
go get github.com/tonyd33/conduit/sdk/go
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    conduit "github.com/tonyd33/conduit/sdk/go"
)

func main() {
    // Create client (automatically loads config from environment)
    client, err := conduit.NewClient()
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Define your message handler
    handler := func(ctx context.Context, msg *conduit.Message) error {
        // Process the message
        var payload string
        if err := msg.UnmarshalPayload(&payload); err != nil {
            return err
        }

        log.Printf("Received: %s", payload)

        // Publish response
        return client.Publish(ctx, map[string]string{
            "response": "processed",
        })
    }

    // Start processing messages
    ctx := context.Background()
    if err := client.Run(ctx, handler); err != nil {
        log.Fatalf("Client error: %v", err)
    }
}
```

## Core Concepts

### Client

The `Client` is the main interface to the Conduit SDK. It handles:
- NATS connection management
- Message subscription and publishing
- State management and checkpointing
- Control message handling (pause/resume)

```go
client, err := conduit.NewClient()
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Messages

Messages follow a standard envelope format:

```go
type Message struct {
    ID        string            // Unique message identifier
    Timestamp time.Time         // When the message was created
    Sequence  uint64            // NATS sequence number
    Type      MessageType       // Message type (Data, Control, Error)
    Payload   json.RawMessage   // The actual message payload
    Metadata  map[string]string // Optional metadata
}
```

Unmarshal the payload into your desired type:

```go
var data MyDataType
if err := msg.UnmarshalPayload(&data); err != nil {
    return err
}
```

### Message Handler

Your application logic is implemented as a `MessageHandler`:

```go
type MessageHandler func(context.Context, *Message) error
```

The handler should:
- Process the incoming message
- Return an error if processing fails (message will be redelivered)
- Return nil on success (message will be acknowledged)

### State Management

The SDK provides built-in state management:

```go
type AppState struct {
    Counter int `json:"counter"`
    Data    map[string]string `json:"data"`
}

state := &AppState{Counter: 0}
client.UpdateState(state)

// In your handler
state.Counter++
client.UpdateState(state)
```

### Checkpointing

Periodically save your state to enable recovery:

```go
// In your message handler
if state.Counter % 100 == 0 {
    if err := client.Checkpoint(ctx); err != nil {
        log.Printf("Checkpoint failed: %v", err)
    }
}
```

When your pod restarts, the SDK will:
1. Load the last checkpoint from the Exchange status
2. Restore your application state
3. Resume processing from the last acknowledged message

### Publishing Messages

Publish messages to the output subject:

```go
// Publish any JSON-serializable value
err := client.Publish(ctx, map[string]interface{}{
    "result": "success",
    "count": 42,
})
```

## Configuration

The SDK automatically loads configuration from environment variables injected by the operator:

| Variable | Description |
|----------|-------------|
| `EXCHANGE_ID` | Unique identifier for this Exchange instance |
| `EXCHANGE_NAME` | Name of the Exchange resource |
| `EXCHANGE_NAMESPACE` | Kubernetes namespace |
| `NATS_URL` | NATS server URL |
| `NATS_STREAM_NAME` | JetStream stream name |
| `NATS_CONSUMER_NAME` | Durable consumer name |
| `NATS_SUBJECT_INPUT` | Subject to receive messages from |
| `NATS_SUBJECT_OUTPUT` | Subject to publish messages to |
| `NATS_SUBJECT_CONTROL` | Subject for control messages |
| `CHECKPOINT_DATA` | Last checkpoint data (JSON) |
| `IS_RECOVERING` | Whether this is a recovery from checkpoint |

You typically don't need to set these manually - the operator handles it.

## Complete Example

See the [echo example](./examples/echo) for a complete working application that demonstrates:

- Message handling
- State management
- Periodic checkpointing
- Graceful shutdown
- Signal handling

## Running Locally

For local development without Kubernetes:

```bash
# Start NATS with JetStream
docker run -p 4222:4222 nats:latest -js

# Set required environment variables
export EXCHANGE_ID="local-test"
export EXCHANGE_NAME="test"
export EXCHANGE_NAMESPACE="default"
export NATS_URL="nats://localhost:4222"
export NATS_STREAM_NAME="test-stream"
export NATS_CONSUMER_NAME="test-consumer"
export NATS_SUBJECT_INPUT="exchange.test.input"
export NATS_SUBJECT_OUTPUT="exchange.test.output"
export NATS_SUBJECT_CONTROL="exchange.test.control"

# Run your application
go run main.go
```

## Deploying to Kubernetes

1. Build your application as a container image:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/app .
CMD ["./app"]
```

2. Create an Exchange resource:

```yaml
apiVersion: conduit.mnke.org/v1alpha1
kind: Exchange
metadata:
  name: my-exchange
spec:
  image: "your-registry/your-app:latest"
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "200m"
  stream:
    retention:
      maxAge: "24h"
      maxMessages: 10000
```

3. Apply the Exchange:

```bash
kubectl apply -f exchange.yaml
```

The operator will:
- Create the NATS stream and consumer
- Deploy your application as a pod
- Inject all necessary configuration
- Handle recovery if the pod restarts

## API Reference

### Client

#### `NewClient() (*Client, error)`
Creates a new Conduit client, loading configuration from environment variables.

#### `(*Client) Run(ctx context.Context, handler MessageHandler) error`
Starts processing messages with the provided handler. Blocks until context is cancelled or an error occurs.

#### `(*Client) Publish(ctx context.Context, payload interface{}) error`
Publishes a message to the output subject. The payload must be JSON-serializable.

#### `(*Client) UpdateState(state interface{})`
Updates the internal state. This state will be saved during checkpointing.

#### `(*Client) Checkpoint(ctx context.Context) error`
Saves the current state and last processed sequence number to the Exchange status.

#### `(*Client) Close() error`
Closes the NATS connection and releases resources. Should be called when shutting down.

### Message

#### `(*Message) UnmarshalPayload(v interface{}) error`
Unmarshals the message payload into the provided value.

#### `NewMessage(msgType MessageType, payload interface{}) (*Message, error)`
Creates a new message with the given type and payload.

### MessageType

```go
const (
    MessageTypeData    MessageType = "data"
    MessageTypeControl MessageType = "control"
    MessageTypeError   MessageType = "error"
)
```

## Best Practices

### Error Handling

Return errors from your handler when processing fails:

```go
handler := func(ctx context.Context, msg *conduit.Message) error {
    if err := processMessage(msg); err != nil {
        // Message will be redelivered
        return fmt.Errorf("processing failed: %w", err)
    }
    return nil // Message acknowledged
}
```

### Checkpoint Frequency

Checkpoint frequently enough to minimize reprocessing, but not so often that it impacts performance:

```go
// Good: Checkpoint every N messages
if state.ProcessedCount % 100 == 0 {
    client.Checkpoint(ctx)
}

// Also good: Checkpoint every N seconds
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        client.Checkpoint(ctx)
    }
}()
```

### Graceful Shutdown

Always handle shutdown signals:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    log.Println("Shutting down...")
    cancel()
}()

client.Run(ctx, handler)
```

### Idempotency

Design your handlers to be idempotent since messages may be redelivered:

```go
handler := func(ctx context.Context, msg *conduit.Message) error {
    // Check if already processed
    if state.ProcessedIDs[msg.ID] {
        return nil // Already handled
    }

    // Process message
    if err := processMessage(msg); err != nil {
        return err
    }

    // Mark as processed
    state.ProcessedIDs[msg.ID] = true
    client.UpdateState(state)

    return nil
}
```

## Troubleshooting

### Connection Issues

If the client fails to connect to NATS, check:
- NATS server is running and accessible
- `NATS_URL` environment variable is set correctly
- Network policies allow pod-to-NATS communication

### Messages Not Being Received

Check:
- The stream exists: `nats stream ls`
- The consumer exists: `nats consumer ls <stream-name>`
- Messages are being published to the correct subject

### Recovery Not Working

Ensure:
- You're calling `client.Checkpoint()` periodically
- Your state struct has JSON tags
- The Exchange CRD has RBAC permissions to update status

## Contributing

Contributions are welcome! Please see the main project [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](../../LICENSE) in the main project repository.
