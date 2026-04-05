# Interactive Conduit Client Example

This example demonstrates the full lifecycle of working with Conduit Exchanges:

1. **Create** an Exchange via the Conduit API
2. **Wait** for the Exchange to become ready
3. **Connect** to the Exchange's NATS subjects
4. **Send** messages interactively from user input
5. **Receive** and display responses from the Exchange worker
6. **Delete** the Exchange when the user exits

## Prerequisites

- Conduit operator running with API server enabled
- NATS server running
- Echo worker image available

## Usage

```bash
# Build and run
go run main.go

# With custom configuration
API_URL=http://localhost:8090 \
WORKER_IMAGE=ghcr.io/tonyd33/conduit-worker:latest \
EXCHANGE_NAME=my-exchange \
go run main.go
```

## How It Works

### 1. Exchange Creation

The client calls the Conduit API to create an Exchange:

```go
client, err := conduit.CreateExchange(apiURL, conduit.ExchangeRequest{
    Name:      "interactive-exchange",
    Namespace: "default",
    Image:     "ghcr.io/tonyd33/conduit-worker:latest",
}, "")
```

The `CreateExchange` function:
- Creates the Exchange via POST `/api/v1/exchanges`
- Waits for the Exchange to become ready
- Automatically retrieves NATS connection info (URL, subjects)
- Returns a fully connected client

### 2. Message Communication

Once connected, you can send messages:

```go
// Send message
client.Send(ctx, "Hello, Exchange!")

// Subscribe to responses
client.Subscribe(ctx, func(msg *conduit.Message) error {
    fmt.Printf("Response: %s\n", string(msg.Payload))
    return nil
})
```

### 3. Cleanup

When you exit (Ctrl+D or Ctrl+C), the client:

```go
// Close NATS connection
client.Close()

// Delete the Exchange via API
client.DeleteExchange()
```

## Example Session

```
Conduit Interactive Client
===========================
API Server: http://localhost:8090
Worker Image: ghcr.io/tonyd33/conduit-worker:latest
Exchange Name: interactive-exchange

Creating Exchange...
Exchange created and ready!

You can now send messages to the Exchange.
Type your message and press Enter. Press Ctrl+D or Ctrl+C to exit.

>> Hello!
  << Echo: Hello!
>> How are you?
  << Echo: How are you?
>> ^D

Cleaning up...
Closing connection...
Deleting Exchange...
Exchange deleted successfully
```

## Environment Variables

- `API_URL`: Conduit API server URL (default: `http://localhost:8090`)
- `WORKER_IMAGE`: Docker image for the Exchange worker (default: `ghcr.io/tonyd33/conduit-worker:latest`)
- `EXCHANGE_NAME`: Name for the Exchange (default: `interactive-exchange`)

## Key Features

- **Unified Client**: Single client handles both API calls and NATS communication
- **Automatic Cleanup**: Exchange is deleted when program exits
- **Interactive**: Real-time message exchange with the worker
- **Simple**: No Kubernetes knowledge required - just an API URL
