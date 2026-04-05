"""
Main Conduit SDK client.

Provides async interface for message processing and publishing.
"""

import asyncio
import logging
from typing import Any, Callable, Coroutine, Optional

import nats
from nats.js import JetStreamContext
from nats.js.api import ConsumerConfig, AckPolicy

from .config import Config
from .message import Message, MessageType

logger = logging.getLogger(__name__)

# Type alias for message handler
MessageHandler = Callable[[Message], Coroutine[Any, Any, None]]


class Client:
    """
    Conduit SDK client for Exchange-compatible applications.

    Provides async interface for:
    - Receiving messages from NATS JetStream
    - Publishing messages to output subject
    - Handling control messages

    Example:
        ```python
        import asyncio
        from conduit_sdk import Client, Message

        async def handler(msg: Message):
            payload = msg.payload
            print(f"Received: {payload}")
            await client.publish({"response": "processed"})

        async def main():
            client = await Client.create()
            try:
                await client.run(handler)
            finally:
                await client.close()

        asyncio.run(main())
        ```
    """

    def __init__(
        self,
        config: Config,
        nc: nats.NATS,
        js: JetStreamContext,
    ):
        """
        Initialize the client.

        Use Client.create() instead of calling this directly.
        """
        self.config = config
        self.nc = nc
        self.js = js
        self.paused: bool = False
        self._subscription = None

    @classmethod
    async def create(cls, config: Optional[Config] = None) -> "Client":
        """
        Create a new Conduit client.

        Args:
            config: Configuration object (loads from env if not provided)

        Returns:
            Initialized Client instance

        Raises:
            Exception: If connection to NATS fails
        """
        if config is None:
            config = Config.from_env()

        logger.info(f"Connecting to NATS at {config.nats_url}")

        # Connect to NATS
        nc = await nats.connect(config.nats_url)
        js = nc.jetstream()

        client = cls(config, nc, js)

        return client

    async def close(self) -> None:
        """Close the NATS connection."""
        if self._subscription:
            await self._subscription.unsubscribe()
        await self.nc.close()
        logger.info("Client closed")

    async def publish(
        self,
        payload: Any,
        metadata: Optional[dict] = None,
    ) -> None:
        """
        Publish a message to the output subject.

        Args:
            payload: Message payload (must be JSON-serializable)
            metadata: Optional metadata dictionary

        Raises:
            Exception: If publishing fails
        """
        msg = Message.create(MessageType.DATA, payload, metadata)
        data = msg.to_json()

        await self.js.publish(self.config.subject_output, data)
        logger.debug(f"Published message to {self.config.subject_output}")

    async def _handle_control_message(self, msg: Message) -> None:
        """
        Handle control messages from the operator.

        Args:
            msg: Control message
        """
        payload = msg.payload
        if isinstance(payload, dict):
            command = payload.get("command")
            if command == "pause":
                self.paused = True
                logger.info("Application paused by control message")
            elif command == "resume":
                self.paused = False
                logger.info("Application resumed by control message")

    async def run(self, handler: MessageHandler) -> None:
        """
        Start processing messages with the provided handler.

        This method blocks until the client is closed or an error occurs.

        Args:
            handler: Async function that processes messages

        Raises:
            Exception: If message processing fails
        """
        logger.info(f"Starting message processing on {self.config.subject_input}")

        async def message_callback(nats_msg):
            """Internal callback for NATS messages."""
            try:
                # Parse the message
                msg = Message.from_json(nats_msg.data)
                msg.sequence = nats_msg.metadata.sequence.stream

                # Handle control messages
                if msg.type == MessageType.CONTROL:
                    await self._handle_control_message(msg)
                    await nats_msg.ack()
                    return

                # Skip processing if paused
                if self.paused:
                    logger.debug(f"Skipping message {msg.sequence} (paused)")
                    return

                # Call user handler
                await handler(msg)

                # Acknowledge the message
                await nats_msg.ack()
                logger.debug(f"Processed message sequence {msg.sequence}")

            except Exception as e:
                logger.error(f"Error processing message: {e}", exc_info=True)
                # NAK the message so it can be redelivered
                await nats_msg.nak()

        # Subscribe to the consumer
        self._subscription = await self.js.subscribe(
            self.config.subject_input,
            stream=self.config.stream_name,
            durable=self.config.consumer_name,
            cb=message_callback,
            manual_ack=True,
        )

        logger.info("Ready to process messages")

        # Keep running until interrupted
        try:
            # Wait indefinitely
            while True:
                await asyncio.sleep(1)
        except asyncio.CancelledError:
            logger.info("Message processing cancelled")
            raise

    async def __aenter__(self):
        """Async context manager entry."""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit."""
        await self.close()
