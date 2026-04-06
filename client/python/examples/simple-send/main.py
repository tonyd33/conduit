#!/usr/bin/env python3
"""
Simple example of creating an Exchange and sending messages using the Conduit Python client.

This demonstrates:
1. Creating an Exchange via Kubernetes API
2. Sending messages to the Exchange
3. Subscribing to responses
4. Cleaning up resources
"""

import asyncio
import os

from conduit_client import Conduit, ExchangeRequest


async def main():
    # Configuration from environment variables
    worker_image = os.getenv("WORKER_IMAGE", "conduit/examples/echo:go")
    exchange_name = os.getenv("EXCHANGE_NAME", "simple-send-py")
    namespace = os.getenv("NAMESPACE", "default")
    nats_url = os.getenv("NATS_URL", "nats://conduit-nats.conduit-system.svc.cluster.local:4222")

    print("Conduit Simple Send Example (Python)")
    print("=" * 40)
    print(f"Namespace: {namespace}")
    print(f"NATS URL: {nats_url}")
    print(f"Worker Image: {worker_image}")
    print(f"Exchange Name: {exchange_name}\n")

    conduit = Conduit(nats_url=nats_url, namespace=namespace)

    # Create Exchange via Kubernetes API
    print("Creating Exchange...")
    exchange = await conduit.create_exchange_client(
        ExchangeRequest(
            name=exchange_name,
            namespace=namespace,
            image=worker_image,
        )
    )
    print("Exchange created and ready!\n")

    # Subscribe to responses
    response_received = asyncio.Event()

    async def handle_response(msg):
        print(f"Received response: {msg.payload}")
        response_received.set()

    # Start subscription in background
    subscription_task = asyncio.create_task(exchange.subscribe(handle_response))

    try:
        # Send a message
        print("Sending message...")
        await exchange.send({"message": "Hello from Python!"})
        print("Message sent!\n")

        # Wait for response with timeout
        try:
            await asyncio.wait_for(response_received.wait(), timeout=10.0)
        except asyncio.TimeoutError:
            print("No response received within timeout")

    finally:
        # Clean up
        print("\nCleaning up...")

        # Cancel subscription
        subscription_task.cancel()
        try:
            await subscription_task
        except asyncio.CancelledError:
            pass

        # Close connection
        print("Closing connection...")
        await exchange.close()

        # Delete Exchange
        print("Deleting Exchange...")
        await conduit.delete_exchange_client(exchange)
        print("Done!")


if __name__ == "__main__":
    asyncio.run(main())
