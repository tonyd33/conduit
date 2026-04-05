# Conduit

A Kubernetes operator for managing stateful, bidirectional workloads with durable message streams and automatic recovery.

## Overview

Conduit provides a framework for running stateful applications that process messages bidirectionally through NATS JetStream. It handles the complexities of stream management, pod lifecycle, failure recovery, and checkpointing, allowing you to focus on your application logic.

**Key Features:**
- **Durable Message Streams**: Built on NATS JetStream for reliable message delivery
- **Automatic Recovery**: Handles pod failures with configurable restart policies and exponential backoff
- **Checkpoint-based State**: Resume processing from saved checkpoints after restarts
- **Multi-language SDKs**: Official SDKs for Go, Python, and TypeScript
- **External Client Libraries**: Send messages to and receive responses from workers
- **REST API**: Manage exchanges programmatically with OpenAPI/Swagger documentation
- **Prometheus Metrics**: Built-in observability for streams, consumers, and exchange lifecycle
- **Validation Webhooks**: Automatic defaulting and validation of Exchange resources

## Use Cases

Conduit is designed for **any bidirectional workload** that requires reliable message processing and state recovery:

- **AI Agents**: Conversational AI with long-running state
- **Data Processing Pipelines**: Interactive ETL workflows with feedback loops
- **Stateful Workflow Engines**: Business process automation
- **Request-Response Systems**: Microservices with durable queuing
- **Real-time Streaming**: Event processing with exactly-once semantics

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     External Clients                        │
│  (Go/Python/TypeScript Client Libraries or HTTP API)       │
└─────────────────────┬───────────────────────────────────────┘
                      │ Send messages / Create Exchanges
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    Conduit Operator                         │
│  ┌───────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   Exchange    │  │  API Server  │  │    Metrics      │  │
│  │  Controller   │  │  (REST API)  │  │   Collector     │  │
│  └───────┬───────┘  └──────────────┘  └─────────────────┘  │
└──────────┼──────────────────────────────────────────────────┘
           │ Manages Pods & NATS Resources
           ▼
┌─────────────────────────────────────────────────────────────┐
│                     NATS JetStream                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│  │   Stream   │  │  Consumer  │  │  Subjects  │           │
│  │ (Messages) │  │  (Durable) │  │  (Topics)  │           │
│  └────────────┘  └────────────┘  └────────────┘           │
└──────────┬──────────────────────────────────────────────────┘
           │ Pull messages & Publish responses
           ▼
┌─────────────────────────────────────────────────────────────┐
│                   Worker Pods                               │
│  (Using Go/Python/TypeScript SDK)                          │
│  - Pull messages from consumer                             │
│  - Process with application logic                          │
│  - Publish responses to output subject                     │
│  - Save checkpoints periodically                           │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.11.3+)
- `kubectl` (v1.11.3+)
- NATS JetStream server
- Go 1.24.0+ (for development)

### Installation

1. **Install the CRDs:**

```bash
make install
```

2. **Deploy NATS JetStream** (if not already running):

```bash
# Example using Helm
helm repo add nats https://nats-io.github.io/k8s/helm/charts/
helm install nats nats/nats --set nats.jetstream.enabled=true
```

3. **Deploy the Conduit Operator:**

```bash
# Build and push the image
make docker-build docker-push IMG=<your-registry>/conduit:latest

# Deploy to cluster
make deploy IMG=<your-registry>/conduit:latest
```

4. **Configure the operator** with NATS connection:

```bash
# Edit the deployment to add NATS URL flag
kubectl edit deployment -n conduit-system conduit-controller-manager

# Add to container args:
# - --nats-url=nats://nats.default.svc:4222
```

### Creating Your First Exchange

```yaml
apiVersion: conduit.mnke.org/v1alpha1
kind: Exchange
metadata:
  name: my-first-exchange
  namespace: default
spec:
  # Container image for your worker
  image: my-worker-image:latest

  # Optional: Resource requirements
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "200m"

  # Optional: Stream configuration (defaults provided)
  stream:
    retention:
      maxAge: 24h
      maxMessages: 100000
    subjects:
      input: "my.app.input"
      output: "my.app.output"

  # Optional: Recovery configuration
  recovery:
    maxRestarts: 5
    checkpointInterval: 5m
```

Apply the Exchange:

```bash
kubectl apply -f my-exchange.yaml
```

Check the status:

```bash
kubectl get exchange my-first-exchange
kubectl describe exchange my-first-exchange
```

## Using the SDKs

Conduit provides official SDKs for building worker applications:

### Go SDK

```go
package main

import (
    "context"
    "log"
    conduit "github.com/tonyd33/conduit/sdk/go"
)

func main() {
    client, err := conduit.NewClient(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    err = client.ProcessMessages(func(msg *conduit.Message) (*conduit.Message, error) {
        // Your processing logic here
        return &conduit.Message{
            Type:    conduit.MessageTypeData,
            Content: "Echo: " + msg.Content,
        }, nil
    })

    if err != nil {
        log.Fatal(err)
    }
}
```

See [sdk/go/README.md](sdk/go/README.md) for full documentation.

### Python SDK

```python
import asyncio
from conduit_sdk import Client, Message, MessageType

async def process_message(msg: Message) -> Message:
    # Your processing logic here
    return Message(
        type=MessageType.DATA,
        content=f"Echo: {msg.content}"
    )

async def main():
    async with Client() as client:
        await client.process_messages(process_message)

if __name__ == "__main__":
    asyncio.run(main())
```

See [sdk/python/README.md](sdk/python/README.md) for full documentation.

### TypeScript SDK

```typescript
import { Client, Message, MessageType } from 'conduit-sdk';

async function processMessage(msg: Message): Promise<Message> {
  // Your processing logic here
  return {
    type: MessageType.DATA,
    content: `Echo: ${msg.content}`,
  };
}

async function main() {
  const client = await Client.connect();

  try {
    await client.processMessages(processMessage);
  } finally {
    await client.close();
  }
}

main().catch(console.error);
```

See [sdk/typescript/README.md](sdk/typescript/README.md) for full documentation.

## Using the Client Libraries

External applications can interact with Exchanges using client libraries:

### Go Client

```go
package main

import (
    "context"
    "log"
    conduit "github.com/tonyd33/conduit/client/go"
)

func main() {
    // Create exchange and get connected client
    client, err := conduit.CreateExchange(context.Background(), conduit.CreateExchangeOptions{
        Name:      "my-exchange",
        Namespace: "default",
        Image:     "my-worker:latest",
        APIServerURL: "http://conduit-api:8090",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Send a message and wait for response
    response, err := client.Request(context.Background(), "Hello, world!", 30*time.Second)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Response: %s", response.Content)
}
```

See [client/go/README.md](client/go/README.md) for full documentation.

## REST API

The Conduit operator includes a REST API for managing Exchanges:

### Starting the API Server

Add flags when deploying the operator:

```bash
--api-server-address=:8090
```

For API authentication, set the `API_SERVER_KEY` environment variable:

```bash
export API_SERVER_KEY=your-secret-api-key
```

### API Documentation

Interactive Swagger UI available at: `http://<api-server>:8090/swagger/`

**Endpoints:**
- `GET /api/v1/exchanges` - List all exchanges
- `POST /api/v1/exchanges` - Create a new exchange
- `GET /api/v1/exchanges/{namespace}/{name}` - Get exchange details
- `DELETE /api/v1/exchanges/{namespace}/{name}` - Delete an exchange
- `GET /healthz` - Health check

**Authentication:**

```bash
curl -H "Authorization: Bearer your-secret-api-key" \
  http://localhost:8090/api/v1/exchanges
```

## Monitoring

### Prometheus Metrics

The operator exposes metrics at `:8080/metrics`:

```bash
# Exchange lifecycle metrics
conduit_exchange_phase{phase="Running"} 5
conduit_exchange_restarts_total{namespace="default",name="my-exchange"} 2

# NATS stream metrics
conduit_stream_messages{stream="stream-exchange-abc123"} 1000
conduit_stream_bytes{stream="stream-exchange-abc123"} 5242880

# Consumer metrics
conduit_consumer_delivered{consumer="exchange-abc123-consumer"} 950
conduit_consumer_pending{consumer="exchange-abc123-consumer"} 50
```

### Events

View Kubernetes events for observability:

```bash
kubectl get events --sort-by='.lastTimestamp'
```

Events include:
- StreamCreated, ConsumerCreated, PodCreated
- Running, Recovering, BackoffWaiting
- StreamCreationFailed, PodFailed, MaxRestartsExceeded

## Configuration

### Operator Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--nats-url` | NATS server URL | "" (disabled) |
| `--nats-nkey-file` | Path to NKey file | "" |
| `--nats-credentials-file` | Path to JWT credentials | "" |
| `--nats-tls-cert-file` | Client certificate | "" |
| `--nats-tls-key-file` | Client key | "" |
| `--nats-tls-ca-file` | CA certificate | "" |
| `--api-server-address` | API server bind address | "" (disabled) |
| `--metrics-bind-address` | Metrics endpoint | :8080 |
| `--health-probe-bind-address` | Health probe endpoint | :8081 |

### Environment Variables (for sensitive credentials)

For security, sensitive credentials should be passed via environment variables, not command-line flags:

| Variable | Description |
|----------|-------------|
| `NATS_USERNAME` | NATS username for basic authentication |
| `NATS_PASSWORD` | NATS password for basic authentication |
| `NATS_TOKEN` | NATS token for token-based authentication |
| `API_SERVER_KEY` | API key for API server authentication |

### Exchange Spec Fields

See [DESIGN.md](DESIGN.md) for complete API reference.

**Common fields:**
- `image` (required): Container image for the worker
- `imagePullPolicy`: Image pull policy (default: IfNotPresent)
- `resources`: Resource requests and limits
- `env`: Environment variables for the worker
- `stream.subjects`: NATS subjects for input/output/control
- `stream.retention`: Message retention policy
- `recovery.maxRestarts`: Maximum restart attempts (default: 5)
- `recovery.checkpointInterval`: Checkpoint save interval (default: 5m)

## Advanced Features

### Checkpointing

Workers can save state for recovery:

```go
client.SaveCheckpoint(context.Background(), checkpointData)
```

After a restart, the checkpoint is available:

```go
checkpoint := client.LoadCheckpoint()
if checkpoint != nil {
    // Resume from checkpoint
}
```

### Validation Webhooks

Exchanges are automatically validated:
- Required fields (image must be specified)
- Positive values for retention and recovery settings
- Valid NATS subject names
- No reserved environment variable prefixes

### Recovery with Exponential Backoff

Failed pods are automatically restarted with exponential backoff:
- First restart: 10 seconds
- Second restart: 20 seconds
- Third restart: 40 seconds
- Maximum backoff: 5 minutes

## Development

### Running Locally

```bash
# Install CRDs
make install

# Run operator locally (requires KUBECONFIG)
make run -- --nats-url=nats://localhost:4222

# Or with all flags
make run -- \
  --nats-url=nats://localhost:4222 \
  --api-server-address=:8090 \
  --metrics-secure=false
```

### Building

```bash
# Generate manifests and code
make manifests generate

# Build binary
make build

# Run tests
make test

# Build Docker image
make docker-build IMG=<registry>/conduit:tag
```

### Generating OpenAPI Spec

After modifying API server annotations:

```bash
swag init -g internal/apiserver/server.go --parseDependency --parseInternal
```

## Documentation

- [DESIGN.md](DESIGN.md) - Architecture and design decisions
- [PROGRESS.md](PROGRESS.md) - Development progress and status
- [sdk/go/README.md](sdk/go/README.md) - Go SDK documentation
- [sdk/python/README.md](sdk/python/README.md) - Python SDK documentation
- [sdk/typescript/README.md](sdk/typescript/README.md) - TypeScript SDK documentation
- [client/go/README.md](client/go/README.md) - Go client library documentation
- [client/python/README.md](client/python/README.md) - Python client library documentation
- [client/typescript/README.md](client/typescript/README.md) - TypeScript client library documentation

## Examples

See the `sdk/*/examples/` and `client/*/examples/` directories for complete examples:

- **Echo Worker**: Simple request-response echo service
- **Interactive Client**: Full lifecycle management with Exchange creation

## Troubleshooting

### Exchange stuck in Pending

```bash
# Check pod status
kubectl get pods -l exchange-id=<exchange-uid>

# Check operator logs
kubectl logs -n conduit-system deployment/conduit-controller-manager

# Check NATS connection
kubectl describe exchange <name>
```

### Messages not processing

```bash
# Check stream exists
nats stream info stream-exchange-<uid>

# Check consumer exists
nats consumer info stream-exchange-<uid> exchange-<uid>-consumer

# Check worker logs
kubectl logs <worker-pod-name>
```

### Frequent restarts

```bash
# Check restart count and backoff status
kubectl describe exchange <name>

# View events
kubectl get events --field-selector involvedObject.name=<exchange-name>

# Check if maxRestarts limit reached
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

Copyright 2026 tonyd33.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
