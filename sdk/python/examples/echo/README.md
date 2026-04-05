# Echo Exchange Example (Python)

This is a simple example application that demonstrates how to use the Conduit Python SDK to build an Exchange-compatible workload.

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

# Install the SDK (from the sdk/python directory)
cd ../../
pip install -e .
cd examples/echo

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
python main.py
```

## Building Docker image

```bash
docker build -t echo-exchange-python:latest .
```

## Deploying to Kubernetes with Conduit

1. Build and push the Docker image:
```bash
docker build -t <your-registry>/echo-exchange-python:latest .
docker push <your-registry>/echo-exchange-python:latest
```

2. Create an Exchange resource:
```yaml
apiVersion: conduit.mnke.org/v1alpha1
kind: Exchange
metadata:
  name: echo-example-python
spec:
  image: "<your-registry>/echo-exchange-python:latest"
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
- [x] Async/await message handling
- [x] Type-safe message unmarshaling
- [x] Publishing output messages
- [x] State management with dataclasses
- [x] Periodic checkpointing
- [x] Graceful shutdown handling
- [x] Signal handling (SIGINT, SIGTERM)
- [x] Structured logging
