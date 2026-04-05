"""
Echo Exchange Example

A simple example application demonstrating the Conduit Python SDK.
"""

import asyncio
import logging
import signal

from conduit_sdk import Client, Message

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


async def main():
    """Main application entrypoint."""
    logger.info("Starting Echo Exchange application...")

    # Create the Conduit client
    client = await Client.create()

    # Flag for graceful shutdown
    shutdown_event = asyncio.Event()

    # Handle shutdown signals
    def signal_handler(sig, frame):
        logger.info(f"Received signal {sig}, shutting down...")
        shutdown_event.set()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Define message handler
    async def handler(msg: Message):
        """Process incoming messages."""
        logger.info(f"Received message (seq={msg.sequence}, type={msg.type.value})")

        # Parse payload
        payload = msg.payload
        if isinstance(payload, str):
            logger.info(f"Payload: {payload}")
        else:
            logger.info(f"Payload: {payload}")

        # Echo the message back
        response = {
            "echo": payload,
            "sequence": msg.sequence,
        }

        await client.publish(response)

    try:
        # Start processing in a task
        processing_task = asyncio.create_task(client.run(handler))

        # Wait for shutdown signal
        await shutdown_event.wait()

        # Cancel processing
        processing_task.cancel()
        try:
            await processing_task
        except asyncio.CancelledError:
            pass

    finally:
        await client.close()
        logger.info("Echo application shut down successfully")


if __name__ == "__main__":
    asyncio.run(main())
