# Conduit TypeScript Client

A unified TypeScript client library for managing Conduit Exchanges and communicating with Exchange workers.

## Features

- **Unified API**: Single client handles both Exchange lifecycle management and message communication
- **API Integration**: Create and manage Exchanges via the Conduit API server
- **NATS Messaging**: Send and receive messages using NATS JetStream
- **Automatic Setup**: Automatically retrieves connection info from API
- **Full Lifecycle**: Create, communicate, and clean up Exchanges with simple API

## Installation

```bash
npm install conduit-client
```

Or with Bun:

```bash
bun add conduit-client
```

## Quick Start: End-to-End with API

The simplest way to use Conduit - create an Exchange and start communicating:

```typescript
import { Conduit } from 'conduit-client';

// Create Conduit API client
const conduit = new Conduit({ apiBaseUrl: 'http://localhost:8090' });

// Create Exchange and get a connected client
const client = await conduit.createExchangeClient({
  name: 'my-exchange',
  namespace: 'default',
  image: 'my-worker-image:latest',
});

// Send a message
await client.send({ message: 'Hello, Exchange!' });

// Clean up
await client.close();
await conduit.deleteExchange('my-exchange', 'default');
```

## Usage Patterns

### Pattern 1: Full Lifecycle with API (Recommended)

Create Exchange via API, communicate, then clean up:

```typescript
import { Conduit } from 'conduit-client';

const conduit = new Conduit({ apiBaseUrl: 'http://localhost:8090' });

// Create Exchange and connect
const client = await conduit.createExchangeClient({
  name: 'my-exchange',
  image: 'worker:latest',
  namespace: 'default',
});

// Use client for messaging...
await client.send({ message: 'Hello!' });

// Clean up
await client.close();
await conduit.deleteExchange('my-exchange', 'default');
```

## Messaging Examples

### Simple Send (Fire and Forget)

```typescript
import { ExchangeClient } from 'conduit-client';

const client = await ExchangeClient.create({
  natsUrl: 'nats://localhost:4222',
  subjectInput: 'exchange.my-exchange.input',
  subjectOutput: 'exchange.my-exchange.output',
});

// Send a message without waiting for response
await client.send({ message: 'Hello, Exchange!' });

await client.close();
```

### Request-Response

```typescript
import { ExchangeClient } from 'conduit-client';

const client = await ExchangeClient.create({
  natsUrl: 'nats://localhost:4222',
  subjectInput: 'exchange.my-exchange.input',
  subjectOutput: 'exchange.my-exchange.output',
});

// Send a message and wait for response (with 5 second timeout)
const response = await client.request(
  { message: 'Hello!' },
  5000 // timeout in milliseconds
);

console.log('Response:', response.payload);

await client.close();
```

### Subscribe to All Responses

```typescript
import { ExchangeClient, Message } from 'conduit-client';

const client = await ExchangeClient.create({
  natsUrl: 'nats://localhost:4222',
  subjectInput: 'exchange.my-exchange.input',
  subjectOutput: 'exchange.my-exchange.output',
});

// Subscribe to all responses (runs until process exits)
await client.subscribe(async (msg: Message) => {
  console.log('Received response:', msg.payload);
});
```

### Send and Subscribe Pattern

```typescript
import { ExchangeClient } from 'conduit-client';

const client = await ExchangeClient.create({
  natsUrl: 'nats://localhost:4222',
  subjectInput: 'exchange.my-exchange.input',
  subjectOutput: 'exchange.my-exchange.output',
});

// Subscribe to responses in background
(async () => {
  await client.subscribe(async (msg) => {
    console.log('Response:', msg.payload);
  });
})();

// Send messages
for (let i = 0; i < 10; i++) {
  await client.send({ count: i });
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

### `Conduit`

The main class for managing Exchanges via the Conduit API.

#### `constructor({ apiBaseUrl }: { apiBaseUrl: string })`

Creates a new Conduit API client.

**Example:**
```typescript
const conduit = new Conduit({ apiBaseUrl: 'http://localhost:8090' });
```

#### `async createExchangeClient(req: ExchangeRequest): Promise<ExchangeClient>`

Creates a new Exchange via the API and returns a fully connected client. This is the easiest way to get started.

**Parameters:**
- `req`: Exchange configuration (name, namespace, image, env vars, etc.)

**Returns:** A connected `ExchangeClient` ready to send/receive messages

**Example:**
```typescript
const client = await conduit.createExchangeClient({
  name: 'my-exchange',
  namespace: 'default',
  image: 'worker:latest',
  env: { KEY: 'value' },
});
```

#### `async deleteExchange(name: string, namespace: string): Promise<void>`

Deletes an Exchange via the API.

**Example:**
```typescript
await conduit.deleteExchange('my-exchange', 'default');
```

### `ExchangeClient`

The client for interacting with a specific Exchange.

### Messaging

#### `async send<T>(payload: T, metadata?: Record<string, string>): Promise<void>`

Sends a message to the Exchange without waiting for a response.

#### `async request<TRequest, TResponse>(payload: TRequest, timeout?: number, metadata?: Record<string, string>): Promise<Message<TResponse>>`

Sends a message and waits for a single response with a timeout (default: 5000ms).

#### `async subscribe<T>(handler: ResponseHandler<T>): Promise<void>`

Subscribes to all responses from the Exchange. The handler will be called for each response. This method blocks until the connection is closed.

#### `async subscribeWithQueue<T>(queueGroup: string, handler: ResponseHandler<T>): Promise<void>`

Subscribes to responses using a NATS queue group for load balancing.

### Connection

#### `async close(): Promise<void>`

Closes the NATS connection and cleans up resources.

### Types

```typescript
interface ExchangeRequest {
  name: string;
  image: string;
  namespace?: string;
  env?: Record<string, string>;
}

interface ExchangeResource {
  name: string;
  namespace: string;
  uid: string;
  phase: string;
  message?: string;
  connectionInfo: ConnectionInfo;
}

interface ConnectionInfo {
  natsURL: string;
  streamName: string;
  inputSubject: string;
  outputSubject: string;
}

interface Message<T = any> {
  id: string;
  timestamp: string;
  sequence: number;
  type: MessageType;
  payload: T;
  metadata?: Record<string, string>;
}

enum MessageType {
  DATA = 'data',
  CONTROL = 'control',
  ERROR = 'error',
}

type ResponseHandler<T = any> = (msg: Message<T>) => Promise<void> | void;
```

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

- Node.js 18+ or Bun
- NATS server with JetStream enabled
- TypeScript 5+ (for development)
