# Conduit Python Client

A unified Python client library for managing Conduit Exchanges and communicating with Exchange workers.

## Features

- **Unified API**: Single client handles both Exchange lifecycle management and message communication
- **API Integration**: Create and manage Exchanges via the Conduit API server
- **NATS Messaging**: Send and receive messages using NATS JetStream
- **Automatic Setup**: Automatically retrieves connection info from API
- **Full Lifecycle**: Create, communicate, and clean up Exchanges with simple API

## Installation

```bash
pip install conduit-client
```

Or from source:

```bash
cd client/python
pip install -e .
```

## Quick Start: End-to-End with API

The simplest way to use Conduit - create an Exchange and start communicating:

```python
import asyncio
from conduit_client import create_exchange, ExchangeRequest

async def main():
    # Create Exchange and get a connected client in one step
    client = await create_exchange(
        "http://localhost:8090",
        ExchangeRequest(
            name="my-exchange",
            namespace="default",
            image="my-worker-image:latest",
        ),
    )

    # Send a message
    await client.send({"message": "Hello, Exchange!"})

    # Clean up
    await client.close()
    await client.delete_exchange()

asyncio.run(main())
```

## Usage Patterns

### Pattern 1: Full Lifecycle with API (Recommended)

Create Exchange via API, communicate, then clean up:

```python
import asyncio
from conduit_client import create_exchange, ExchangeRequest

async def main():
    # Create Exchange and connect
    client = await create_exchange(
        "http://localhost:8090",
        ExchangeRequest(
            name="my-exchange",
            image="worker:latest",
        ),
    )

    # Use client for messaging...
    await client.send({"message": "Hello!"})

    # Clean up
    await client.close()
    await client.delete_exchange()

asyncio.run(main())
```

### Pattern 2: Connect to Existing Exchange via API

Connect to an Exchange that already exists:

```python
import asyncio
from conduit_client import ExchangeClient, ClientConfig

async def main():
    client = await ExchangeClient.create(ClientConfig(
        api_base_url="http://localhost:8090",
        exchange_name="existing-exchange",
        exchange_namespace="default",
    ))

    # NATS URL and subjects are automatically fetched from API
    await client.send({"message": "Hello!"})
    await client.close()

asyncio.run(main())
```

### Pattern 3: Direct NATS Connection (No API)

Connect directly if you already know the NATS subjects:

```python
import asyncio
from conduit_client import ExchangeClient, ClientConfig

async def main():
    client = await ExchangeClient.create(ClientConfig(
        nats_url="nats://localhost:4222",
        subject_input="exchange.my-exchange.input",
        subject_output="exchange.my-exchange.output",
    ))

    await client.send({"message": "Hello!"})
    await client.close()

asyncio.run(main())
```

## Messaging Examples

### Simple Send (Fire and Forget)

```python
import asyncio
from conduit_client import ExchangeClient, ClientConfig

async def main():
    config = ClientConfig(
        nats_url="nats://localhost:4222",
        subject_input="exchange.my-exchange.input",
        subject_output="exchange.my-exchange.output",
    )

    client = await ExchangeClient.create(config)

    # Send a message without waiting for response
    await client.send({"message": "Hello, Exchange!"})

    await client.close()

asyncio.run(main())
```

### Request-Response

```python
import asyncio
from conduit_client import ExchangeClient, ClientConfig

async def main():
    config = ClientConfig(
        nats_url="nats://localhost:4222",
        subject_input="exchange.my-exchange.input",
        subject_output="exchange.my-exchange.output",
    )

    client = await ExchangeClient.create(config)

    # Send a message and wait for response (with 5 second timeout)
    response = await client.request(
        {"message": "Hello!"},
        timeout=5.0
    )

    print(f"Response: {response.payload}")

    await client.close()

asyncio.run(main())
```

### Subscribe to All Responses

```python
import asyncio
from conduit_client import ExchangeClient, ClientConfig, Message

async def handle_response(msg: Message):
    print(f"Received response: {msg.payload}")

async def main():
    config = ClientConfig(
        nats_url="nats://localhost:4222",
        subject_input="exchange.my-exchange.input",
        subject_output="exchange.my-exchange.output",
    )

    client = await ExchangeClient.create(config)

    # Subscribe to all responses
    await client.subscribe(handle_response)

    # Send some messages
    for i in range(10):
        await client.send({"count": i})

    # Keep running
    await asyncio.Event().wait()

asyncio.run(main())
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

#### `async create_exchange(api_base_url: str, req: ExchangeRequest, api_key: str = None) -> ExchangeClient`

Creates a new Exchange via the API and returns a fully connected client. This is the easiest way to get started.

**Parameters:**
- `api_base_url`: Conduit API server URL (e.g., "http://localhost:8090")
- `req`: Exchange configuration (name, namespace, image, env vars, etc.)
- `api_key`: Optional API key for authentication

**Returns:** A connected `ExchangeClient` ready to send/receive messages

**Example:**
```python
client = await create_exchange(
    "http://localhost:8090",
    ExchangeRequest(
        name="my-exchange",
        namespace="default",
        image="worker:latest",
        env={"KEY": "value"},
    ),
)
```

### `ExchangeClient`

#### `async ExchangeClient.create(config: ClientConfig) -> ExchangeClient`

Creates a new client for interacting with an Exchange. Supports three modes:
1. **API mode**: Set `api_base_url` and `exchange_name` to auto-fetch connection info
2. **Direct mode**: Set `nats_url` and subjects for direct connection
3. **Hybrid mode**: Combine API and custom configuration

**Example (API mode):**
```python
client = await ExchangeClient.create(ClientConfig(
    api_base_url="http://localhost:8090",
    exchange_name="existing-exchange",
    exchange_namespace="default",
))
```

**Example (Direct mode):**
```python
client = await ExchangeClient.create(ClientConfig(
    nats_url="nats://localhost:4222",
    subject_input="exchange.abc-123.input",
    subject_output="exchange.abc-123.output",
))
```

#### `async delete_exchange() -> None`

Deletes the Exchange via the API (requires API configuration).

### Messaging

#### `async send(payload: Any, metadata: Optional[Dict[str, str]] = None) -> None`

Sends a message to the Exchange without waiting for a response.

#### `async request(payload: Any, timeout: float = 5.0, metadata: Optional[Dict[str, str]] = None) -> Message`

Sends a message and waits for a single response with a timeout.

#### `async subscribe(handler: ResponseHandler) -> None`

Subscribes to all responses from the Exchange. The handler will be called for each response.

#### `async subscribe_with_queue(queue_group: str, handler: ResponseHandler) -> None`

Subscribes to responses using a NATS queue group for load balancing.

### Connection

#### `async close() -> None`

Closes the NATS connection and cleans up resources.

### `ExchangeRequest`

Request to create a new Exchange via the API.

**Fields:**
- `name`: Exchange name
- `image`: Container image for the Exchange worker
- `namespace`: Kubernetes namespace (defaults to "default")
- `env`: Optional environment variables

### `ClientConfig`

Configuration dataclass with fields:
- `nats_url`: NATS server URL (optional if using API)
- `subject_input`: Input subject for the Exchange (optional if using API)
- `subject_output`: Output subject for the Exchange (optional if using API)
- `api_base_url`: Conduit API server URL (e.g., "http://localhost:8090")
- `api_key`: Optional API key for authentication
- `exchange_name`: Name of Exchange to connect to (for API mode)
- `exchange_namespace`: Namespace of Exchange (defaults to "default")

### `Message`

Message envelope with fields:
- `id`: Unique message identifier (UUID)
- `timestamp`: ISO8601 timestamp
- `sequence`: NATS stream sequence number
- `type`: Message type (data, control, error)
- `payload`: Message content (arbitrary JSON-serializable data)
- `metadata`: Optional metadata dictionary

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

## Requirements

- Python 3.9+
- nats-py >= 2.8.0
