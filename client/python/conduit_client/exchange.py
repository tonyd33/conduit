"""ExchangeClient for interacting with Conduit Exchange workers."""

import asyncio
from typing import Any, Callable, Optional

import nats
from nats.js import JetStreamContext

from .message import Message, new_data_message, validate_message


# Type alias for response handler
ResponseHandler = Callable[[Message], None]


class ExchangeClient:
    """
    Client for interacting with Conduit Exchange workers.

    This client provides methods for:
    - Sending messages to an Exchange
    - Waiting for request-response patterns
    - Subscribing to all responses from an Exchange

    Example:
        ```python
        # Typically created via Conduit.create_exchange_client()
        exchange = await conduit.create_exchange_client(...)

        # Send a message
        await exchange.send({"message": "Hello!"})

        # Request-response pattern
        response = await exchange.request({"query": "ping"}, timeout=5.0)

        # Subscribe to all responses
        await exchange.subscribe(async lambda msg: print(msg.payload))

        # Clean up
        await exchange.close()
        ```
    """

    def __init__(
        self,
        nc: nats.NATS,
        js: JetStreamContext,
        subject_input: str,
        subject_output: str,
        name: str,
        namespace: str,
    ):
        """
        Initialize ExchangeClient.

        Typically you should use Conduit.create_exchange_client() instead of
        calling this constructor directly.

        Args:
            nc: NATS connection
            js: JetStream context
            subject_input: Input subject for sending messages
            subject_output: Output subject for receiving messages
            name: Exchange name
            namespace: Exchange namespace
        """
        self.nc = nc
        self.js = js
        self.subject_input = subject_input
        self.subject_output = subject_output
        self.name = name
        self.namespace = namespace
        self._subscriptions = []

    async def send(self, payload: Any, metadata: Optional[dict[str, str]] = None) -> None:
        """
        Send a message to the Exchange without waiting for a response.

        Args:
            payload: Message payload (must be JSON-serializable)
            metadata: Optional metadata dictionary

        Raises:
            Exception: If publishing fails
        """
        msg = new_data_message(payload, metadata)
        validate_message(msg)

        await self.js.publish(self.subject_input, msg.to_bytes())

    async def request(
        self,
        payload: Any,
        timeout: float = 5.0,
        metadata: Optional[dict[str, str]] = None,
    ) -> Message:
        """
        Send a message and wait for a single response.

        Args:
            payload: Message payload (must be JSON-serializable)
            timeout: Timeout in seconds
            metadata: Optional metadata dictionary

        Returns:
            Response message

        Raises:
            asyncio.TimeoutError: If timeout is exceeded
            Exception: If publishing or receiving fails
        """
        msg = new_data_message(payload, metadata)
        validate_message(msg)

        # Create a future for the response
        response_future = asyncio.Future()

        # Subscribe to output subject temporarily
        async def message_handler(nats_msg):
            try:
                resp_msg = Message.from_bytes(nats_msg.data)
                validate_message(resp_msg)
                if not response_future.done():
                    response_future.set_result(resp_msg)
            except Exception as e:
                if not response_future.done():
                    response_future.set_exception(e)

        sub = await self.nc.subscribe(self.subject_output, cb=message_handler)

        try:
            # Publish the request
            await self.js.publish(self.subject_input, msg.to_bytes())

            # Wait for response with timeout
            response = await asyncio.wait_for(response_future, timeout=timeout)
            return response

        finally:
            # Clean up subscription
            await sub.unsubscribe()

    async def subscribe(self, handler: ResponseHandler) -> None:
        """
        Subscribe to all responses from the Exchange.

        This method blocks until the client is closed.

        Args:
            handler: Async function that processes response messages

        Example:
            ```python
            async def handle_response(msg: Message):
                print(f"Received: {msg.payload}")

            await exchange.subscribe(handle_response)
            ```
        """

        async def message_handler(nats_msg):
            try:
                msg = Message.from_bytes(nats_msg.data)
                validate_message(msg)
                await handler(msg)
            except Exception:
                # Silently ignore errors in handler
                pass

        sub = await self.nc.subscribe(self.subject_output, cb=message_handler)
        self._subscriptions.append(sub)

    async def subscribe_with_queue(self, queue_group: str, handler: ResponseHandler) -> None:
        """
        Subscribe to responses using a NATS queue group.

        This allows multiple clients to load-balance response processing.

        Args:
            queue_group: Name of the queue group
            handler: Async function that processes response messages
        """

        async def message_handler(nats_msg):
            try:
                msg = Message.from_bytes(nats_msg.data)
                validate_message(msg)
                await handler(msg)
            except Exception:
                pass

        sub = await self.nc.subscribe(
            self.subject_output,
            queue=queue_group,
            cb=message_handler,
        )
        self._subscriptions.append(sub)

    async def close(self) -> None:
        """Close the NATS connection and clean up resources."""
        # Unsubscribe from all subscriptions
        for sub in self._subscriptions:
            await sub.unsubscribe()

        # Close NATS connection
        if self.nc:
            await self.nc.close()
