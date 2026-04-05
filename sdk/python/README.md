# Conduit Python SDK

The Conduit Python SDK provides an async, Pythonic interface for building Exchange-compatible applications. It handles all the complexity of NATS JetStream integration, checkpoint management, and graceful lifecycle handling.

## Features

- **Async/Await Support**: Built on asyncio for modern Python applications
- **Automatic Configuration**: Loads all necessary configuration from environment variables injected by the Conduit operator
- **Message Handling**: Type-safe message envelope with automatic serialization
- **State Management**: Built-in support for application state with checkpointing
- **Recovery**: Automatic recovery from the last checkpoint when pods restart
- **Graceful Shutdown**: Proper cleanup of NATS connections and resources
- **Control Messages**: Handle pause/resume commands from the operator

## Installation

```bash
pip install conduit-sdk
```

For Kubernetes integration (checkpoint saving):
```bash
pip install conduit-sdk[kubernetes]
```

## Quick Start

```python
import asyncio
from conduit_sdk import Client, Message

async def handler(msg: Message):
    """Process incoming messages."""
    # Get the payload
    payload = msg.payload
    print(f"Received: {payload}")

    # Publish a response
    await client.publish({"response": "processed"})

async def main():
    # Create client (automatically loads config from environment)
    client = await Client.create()

    try:
        # Start processing messages
        await client.run(handler)
    finally:
        await client.close()

if __name__ == "__main__":
    asyncio.run(main())
```

## Core Concepts

### Client

The `Client` is the main interface to the Conduit SDK. It handles:
- NATS connection management
- Message subscription and publishing
- State management and checkpointing
- Control message handling (pause/resume)

```python
# Create a client (loads config from environment)
client = await Client.create()

# Or provide custom config
from conduit_sdk import Config
config = Config.from_env()
client = await Client.create(config)

# Always close when done
await client.close()

# Or use as async context manager
async with await Client.create() as client:
    await client.run(handler)
```

### Messages

Messages follow a standard envelope format:

```python
from conduit_sdk import Message, MessageType

# Messages have these attributes:
msg.id           # Unique message identifier
msg.timestamp    # ISO 8601 timestamp
msg.sequence     # NATS sequence number
msg.type         # MessageType.DATA, CONTROL, or ERROR
msg.payload      # The actual message data
msg.metadata     # Optional metadata dict
```

Access the payload directly:

```python
async def handler(msg: Message):
    # Payload can be dict, list, string, etc.
    if isinstance(msg.payload, dict):
        value = msg.payload.get("key")
    elif isinstance(msg.payload, str):
        text = msg.payload
```

### Message Handler

Your application logic is implemented as an async message handler:

```python
from conduit_sdk import Message

async def handler(msg: Message) -> None:
    """
    Process a message.

    Raise an exception to NAK the message (it will be redelivered).
    Return normally to ACK the message.
    """
    # Your logic here
    await process_message(msg)
```

The handler should:
- Be an async function
- Take a `Message` as the only parameter
- Raise an exception if processing fails (message will be redelivered)
- Return normally on success (message will be acknowledged)

### State Management

The SDK provides built-in state management:

```python
from dataclasses import dataclass, asdict

@dataclass
class AppState:
    counter: int = 0
    data: dict = None

# Initialize state
state = AppState(counter=0, data={})
client.update_state(asdict(state))

# In your handler
async def handler(msg: Message):
    state.counter += 1
    client.update_state(asdict(state))
```

### Checkpointing

Periodically save your state to enable recovery:

```python
async def handler(msg: Message):
    # Process message
    state.counter += 1
    client.update_state(asdict(state))

    # Checkpoint every 100 messages
    if state.counter % 100 == 0:
        await client.checkpoint()
```

When your pod restarts, the SDK will:
1. Load the last checkpoint from the Exchange status
2. Restore your application state
3. Resume processing from the last acknowledged message

### Publishing Messages

Publish messages to the output subject:

```python
# Publish any JSON-serializable value
await client.publish({"result": "success", "count": 42})

# With metadata
await client.publish(
    payload={"data": "value"},
    metadata={"source": "handler"}
)
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

- Async message handling
- State management with dataclasses
- Periodic checkpointing
- Graceful shutdown
- Signal handling
- Structured logging

## Running Locally

For local development without Kubernetes:

```bash
# Start NATS with JetStream
docker run -p 4222:4222 nats:latest -js

# Install the SDK
pip install conduit-sdk

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
python main.py
```

## Deploying to Kubernetes

1. Build your application as a container image:

```dockerfile
FROM python:3.12-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

CMD ["python", "main.py"]
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

#### `await Client.create(config: Optional[Config] = None) -> Client`
Creates a new Conduit client, loading configuration from environment variables.

#### `await client.close() -> None`
Closes the NATS connection and releases resources.

#### `await client.run(handler: MessageHandler) -> None`
Starts processing messages with the provided handler. Blocks until cancelled or an error occurs.

#### `await client.publish(payload: Any, metadata: Optional[dict] = None) -> None`
Publishes a message to the output subject. The payload must be JSON-serializable.

#### `client.update_state(state: Any) -> None`
Updates the internal state. This state will be saved during checkpointing.

#### `await client.checkpoint() -> None`
Saves the current state and last processed sequence number to the Exchange status.

### Message

#### `Message.payload`
The message payload (can be dict, list, str, int, float, bool, or None).

#### `Message.from_json(data: bytes) -> Message`
Parses a message from JSON bytes.

#### `msg.to_json() -> bytes`
Serializes message to JSON bytes.

### MessageType

```python
from conduit_sdk import MessageType

MessageType.DATA     # Regular data messages
MessageType.CONTROL  # Control messages from operator
MessageType.ERROR    # Error messages
```

### Config

#### `Config.from_env() -> Config`
Loads configuration from environment variables.

## Best Practices

### Error Handling

Raise exceptions from your handler when processing fails:

```python
async def handler(msg: Message):
    try:
        await process_message(msg)
    except ProcessingError as e:
        # Message will be redelivered
        raise
```

### Checkpoint Frequency

Checkpoint frequently enough to minimize reprocessing, but not so often that it impacts performance:

```python
# Good: Checkpoint every N messages
if state.processed_count % 100 == 0:
    await client.checkpoint()

# Also good: Checkpoint every N seconds
async def periodic_checkpoint():
    while True:
        await asyncio.sleep(30)
        await client.checkpoint()

asyncio.create_task(periodic_checkpoint())
```

### Graceful Shutdown

Always handle shutdown signals:

```python
import asyncio
import signal

async def main():
    client = await Client.create()
    shutdown_event = asyncio.Event()

    def signal_handler(sig, frame):
        shutdown_event.set()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Start processing
    task = asyncio.create_task(client.run(handler))

    # Wait for shutdown
    await shutdown_event.wait()

    # Cancel and cleanup
    task.cancel()
    try:
        await task
    except asyncio.CancelledError:
        pass

    await client.close()

asyncio.run(main())
```

### Idempotency

Design your handlers to be idempotent since messages may be redelivered:

```python
async def handler(msg: Message):
    # Check if already processed
    if msg.id in state.processed_ids:
        return  # Already handled

    # Process message
    await process_message(msg)

    # Mark as processed
    state.processed_ids.add(msg.id)
    client.update_state(asdict(state))
```

### Type Safety

Use type hints and dataclasses for better type safety:

```python
from dataclasses import dataclass
from typing import Optional

@dataclass
class MessagePayload:
    action: str
    value: int
    metadata: Optional[dict] = None

async def handler(msg: Message):
    if isinstance(msg.payload, dict):
        # Construct typed object from payload
        payload = MessagePayload(**msg.payload)
        print(f"Action: {payload.action}, Value: {payload.value}")
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
- You're calling `await client.checkpoint()` periodically
- Your state is JSON-serializable (use `asdict()` for dataclasses)
- The Exchange CRD has RBAC permissions to update status

### Import Errors

If you get import errors:
```bash
# Make sure nats-py is installed
pip install nats-py

# For Kubernetes integration
pip install kubernetes
```

## Development

To work on the SDK itself:

```bash
# Clone the repository
git clone https://github.com/tonyd33/conduit.git
cd conduit/sdk/python

# Install in editable mode with dev dependencies
pip install -e ".[dev,kubernetes]"

# Run tests
pytest

# Format code
black .
ruff check .
```

## Contributing

Contributions are welcome! Please see the main project [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](../../LICENSE) in the main project repository.
