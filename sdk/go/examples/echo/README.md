# Echo Exchange Example

This is a simple example application that demonstrates how to use the Conduit SDK to build an Exchange-compatible workload.

## What it does

The echo application:
1. Receives messages on the input subject
2. Logs the message content
3. Publishes an echo response to the output subject
4. Tracks message count in its state
5. Automatically checkpoints every 10 messages

## Running locally (without Kubernetes)

You'll need NATS JetStream running locally and to set the required environment variables:

```bash
# Start NATS with JetStream
docker run -p 4222:4222 nats:latest -js

# Set environment variables (these would normally be injected by the operator)
export EXCHANGE_ID="test-exchange-123"
export EXCHANGE_NAME="test-exchange"
export EXCHANGE_NAMESPACE="default"
export NATS_URL="nats://localhost:4222"
export NATS_STREAM_NAME="stream-test-123"
export NATS_CONSUMER_NAME="consumer-test-123"
export NATS_SUBJECT_INPUT="exchange.test.input"
export NATS_SUBJECT_OUTPUT="exchange.test.output"
export NATS_SUBJECT_CONTROL="exchange.test.control"

# Run the application
go run main.go
```

## Building Docker image

```bash
docker build -t echo-exchange:latest .
```

## Deploying to Kubernetes with Conduit

1. Build and push the Docker image:
```bash
docker build -t <your-registry>/echo-exchange:latest .
docker push <your-registry>/echo-exchange:latest
```

2. Create an Exchange resource:
```yaml
apiVersion: conduit.mnke.org/v1alpha1
kind: Exchange
metadata:
  name: echo-example
spec:
  image: "<your-registry>/echo-exchange:latest"
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
      maxMessages: 1000
```

3. Apply the Exchange:
```bash
kubectl apply -f exchange.yaml
```

4. Send messages to test:
```bash
# Using NATS CLI
nats pub exchange.<exchange-id>.input '{"hello": "world"}'

# Subscribe to output
nats sub exchange.<exchange-id>.output
```

## Features demonstrated

- [x] Automatic configuration from environment variables
- [x] Message handling with type-safe unmarshaling
- [x] Publishing output messages
- [x] State management
- [x] Periodic checkpointing
- [x] Graceful shutdown handling
- [x] Signal handling (SIGINT, SIGTERM)
