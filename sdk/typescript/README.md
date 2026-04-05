# Conduit TypeScript SDK

The Conduit TypeScript SDK provides a type-safe, Promise-based interface for building Exchange-compatible applications. It handles all the complexity of NATS JetStream integration, checkpoint management, and graceful lifecycle handling.

## Features

- **Full TypeScript Support**: Complete type definitions for type-safe development
- **Promise/Async-Await**: Modern async patterns for clean, readable code
- **Automatic Configuration**: Loads all necessary configuration from environment variables injected by the Conduit operator
- **Message Handling**: Type-safe message envelope with generic payload support
- **State Management**: Built-in support for application state with checkpointing
- **Recovery**: Automatic recovery from the last checkpoint when pods restart
- **Graceful Shutdown**: Proper cleanup of NATS connections and resources
- **Control Messages**: Handle pause/resume commands from the operator

## Installation

```bash
npm install conduit-sdk
```

## Quick Start

```typescript
import { Client, Message } from 'conduit-sdk';

async function main() {
  // Create client (automatically loads config from environment)
  const client = await Client.create();

  // Define message handler
  const handler = async (msg: Message) => {
    console.log('Received:', msg.payload);
    await client.publish({ response: 'processed' });
  };

  // Start processing messages
  await client.run(handler);
}

main().catch(console.error);
```

## Core Concepts

### Client

The `Client` is the main interface to the Conduit SDK. It handles:
- NATS connection management
- Message subscription and publishing
- State management and checkpointing
- Control message handling (pause/resume)

```typescript
// Create a client (loads config from environment)
const client = await Client.create();

// Or provide custom config
import { loadConfigFromEnv } from 'conduit-sdk';
const config = loadConfigFromEnv();
const client = await Client.create(config);

// Always close when done
await client.close();
```

### Messages

Messages follow a standard envelope format with full TypeScript support:

```typescript
import { Message, MessageType } from 'conduit-sdk';

// Message interface
interface Message<T = any> {
  id: string;          // Unique message identifier
  timestamp: string;   // ISO 8601 timestamp
  sequence: number;    // NATS sequence number
  type: MessageType;   // MessageType.DATA, CONTROL, or ERROR
  payload: T;          // The actual message data (generic type)
  metadata?: Record<string, string>; // Optional metadata
}
```

Access typed payloads:

```typescript
interface MyPayload {
  action: string;
  value: number;
}

const handler = async (msg: Message<MyPayload>) => {
  console.log(`Action: ${msg.payload.action}, Value: ${msg.payload.value}`);
};
```

### Message Handler

Your application logic is implemented as an async message handler:

```typescript
import { Message } from 'conduit-sdk';

const handler = async (msg: Message): Promise<void> => {
  // Your logic here
  await processMessage(msg);

  // Throw an error to NAK (message will be redelivered)
  // Return normally to ACK
};
```

The handler should:
- Be an async function
- Take a `Message` as the only parameter
- Throw an exception if processing fails (message will be redelivered)
- Return normally (or return a Promise) on success (message will be acknowledged)

### State Management

The SDK provides built-in state management:

```typescript
interface AppState {
  counter: number;
  data: Record<string, any>;
}

// Initialize state
const state: AppState = { counter: 0, data: {} };
client.updateState(state);

// In your handler
const handler = async (msg: Message) => {
  state.counter++;
  client.updateState(state);
};
```

### Checkpointing

Periodically save your state to enable recovery:

```typescript
const handler = async (msg: Message) => {
  // Process message
  state.counter++;
  client.updateState(state);

  // Checkpoint every 100 messages
  if (state.counter % 100 === 0) {
    await client.checkpoint();
  }
};
```

When your pod restarts, the SDK will:
1. Load the last checkpoint from the Exchange status
2. Restore your application state
3. Resume processing from the last acknowledged message

### Publishing Messages

Publish messages to the output subject:

```typescript
// Publish any JSON-serializable value
await client.publish({ result: 'success', count: 42 });

// With metadata
await client.publish(
  { data: 'value' },
  { source: 'handler' }
);

// Typed publishing
interface ResponsePayload {
  result: string;
  timestamp: number;
}

const response: ResponsePayload = {
  result: 'processed',
  timestamp: Date.now(),
};
await client.publish(response);
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

- Type-safe message handling
- State management with TypeScript interfaces
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
npm install conduit-sdk

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
npm start
```

## Deploying to Kubernetes

1. Build your application:

```dockerfile
FROM node:20-slim AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

FROM node:20-slim
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
CMD ["npm", "start"]
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

#### `static async create(config?: Config): Promise<Client>`
Creates a new Conduit client, loading configuration from environment variables.

**Parameters:**
- `config` (optional): Custom configuration object

**Returns:** Promise that resolves to an initialized Client instance

**Throws:** Error if connection to NATS fails

#### `async close(): Promise<void>`
Closes the NATS connection and releases resources.

#### `async run<T>(handler: MessageHandler<T>): Promise<void>`
Starts processing messages with the provided handler. Blocks until an error occurs.

**Parameters:**
- `handler`: Async function that processes messages

**Type Parameters:**
- `T`: The expected payload type

#### `async publish<T>(payload: T, metadata?: Record<string, string>): Promise<void>`
Publishes a message to the output subject.

**Parameters:**
- `payload`: Message payload (must be JSON-serializable)
- `metadata` (optional): Metadata dictionary

**Type Parameters:**
- `T`: The payload type

#### `updateState(state: any): void`
Updates the internal state. This state will be saved during checkpointing.

**Parameters:**
- `state`: Application state (must be JSON-serializable)

#### `async checkpoint(): Promise<void>`
Saves the current state and last processed sequence number to the Exchange status.

### Message

```typescript
interface Message<T = any> {
  id: string;
  timestamp: string;
  sequence: number;
  type: MessageType;
  payload: T;
  metadata?: Record<string, string>;
}
```

### MessageType

```typescript
enum MessageType {
  DATA = 'data',     // Regular data messages
  CONTROL = 'control', // Control messages from operator
  ERROR = 'error'    // Error messages
}
```

### MessageHandler

```typescript
type MessageHandler<T = any> = (msg: Message<T>) => Promise<void>;
```

## Best Practices

### Error Handling

Throw exceptions from your handler when processing fails:

```typescript
const handler = async (msg: Message) => {
  try {
    await processMessage(msg);
  } catch (error) {
    // Message will be redelivered
    throw error;
  }
};
```

### Checkpoint Frequency

Checkpoint frequently enough to minimize reprocessing, but not so often that it impacts performance:

```typescript
// Good: Checkpoint every N messages
if (state.processedCount % 100 === 0) {
  await client.checkpoint();
}

// Also good: Checkpoint every N seconds
setInterval(async () => {
  await client.checkpoint();
}, 30000);
```

### Graceful Shutdown

Always handle shutdown signals:

```typescript
async function main() {
  const client = await Client.create();

  const shutdown = async () => {
    console.log('Shutting down...');
    await client.close();
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  await client.run(handler);
}
```

### Idempotency

Design your handlers to be idempotent since messages may be redelivered:

```typescript
const processedIds = new Set<string>();

const handler = async (msg: Message) => {
  // Check if already processed
  if (processedIds.has(msg.id)) {
    return; // Already handled
  }

  // Process message
  await processMessage(msg);

  // Mark as processed
  processedIds.add(msg.id);
  client.updateState({ processedIds: Array.from(processedIds) });
};
```

### Type Safety

Use TypeScript interfaces for type-safe message handling:

```typescript
interface InputPayload {
  action: 'create' | 'update' | 'delete';
  data: Record<string, any>;
}

interface OutputPayload {
  status: 'success' | 'error';
  message: string;
}

const handler = async (msg: Message<InputPayload>) => {
  const { action, data } = msg.payload;

  const response: OutputPayload = {
    status: 'success',
    message: `Processed ${action}`,
  };

  await client.publish(response);
};
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

### Type Errors

Ensure your TypeScript configuration includes:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "esModuleInterop": true,
    "strict": true
  }
}
```

### Recovery Not Working

Ensure:
- You're calling `await client.checkpoint()` periodically
- Your state is JSON-serializable
- The Exchange CRD has RBAC permissions to update status

## Development

To work on the SDK itself:

```bash
# Clone the repository
git clone https://github.com/tonyd33/conduit.git
cd conduit/sdk/typescript

# Install dependencies
npm install

# Build
npm run build

# Watch mode
npm run watch
```

## Contributing

Contributions are welcome! Please see the main project [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](../../LICENSE) in the main project repository.
