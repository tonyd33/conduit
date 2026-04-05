"""Conduit Exchange client for sending and receiving messages."""

import asyncio
import json
from dataclasses import dataclass
from typing import Any, Callable, Optional

import aiohttp
import nats
from nats.js import JetStreamContext

from .message import Message, new_data_message, validate_message


# Type alias for response handler
ResponseHandler = Callable[[Message], None]


@dataclass
class ExchangeRequest:
    """
    Request to create a new Exchange via the API.

    Attributes:
        name: Exchange name
        namespace: Kubernetes namespace (defaults to "default")
        image: Container image for the Exchange worker
        env: Optional environment variables
    """

    name: str
    image: str
    namespace: str = "default"
    env: Optional[dict[str, str]] = None


@dataclass
class ClientConfig:
    """
    Configuration for the Exchange client.

    Attributes:
        nats_url: NATS server URL (optional if using API)
        subject_input: Input subject for sending messages to Exchange (optional if using API)
        subject_output: Output subject for receiving responses from Exchange (optional if using API)
        api_base_url: Conduit API server URL (e.g., "http://localhost:8090")
        api_key: Optional API key for authentication
        exchange_name: Name of Exchange to connect to (for API mode)
        exchange_namespace: Namespace of Exchange (defaults to "default")
    """

    nats_url: Optional[str] = None
    subject_input: Optional[str] = None
    subject_output: Optional[str] = None
    api_base_url: Optional[str] = None
    api_key: Optional[str] = None
    exchange_name: Optional[str] = None
    exchange_namespace: str = "default"


class ExchangeClient:
    """
    Client for interacting with Conduit Exchange workers.

    This client provides methods for:
    - Sending messages to an Exchange
    - Waiting for request-response patterns
    - Subscribing to all responses from an Exchange
    - Creating and deleting Exchanges via the API

    Example (API mode):
        ```python
        # Create Exchange and get connected client
        client = await create_exchange(
            "http://localhost:8090",
            ExchangeRequest(
                name="my-exchange",
                namespace="default",
                image="worker:latest",
            ),
        )

        # Send a message
        await client.send({"message": "Hello!"})

        # Clean up
        await client.close()
        await client.delete_exchange()
        ```

    Example (Direct NATS mode):
        ```python
        config = ClientConfig(
            nats_url="nats://localhost:4222",
            subject_input="exchange.my-exchange.input",
            subject_output="exchange.my-exchange.output",
        )

        client = await ExchangeClient.create(config)
        await client.send({"message": "Hello!"})
        await client.close()
        ```
    """

    def __init__(
        self,
        nc: nats.NATS,
        js: JetStreamContext,
        config: ClientConfig,
    ):
        """Initialize client (use create() or create_exchange() instead)."""
        self.nc = nc
        self.js = js
        self.config = config
        self._subscriptions = []
        self._http_session: Optional[aiohttp.ClientSession] = None

    @classmethod
    async def create(cls, config: ClientConfig) -> "ExchangeClient":
        """
        Create a new Exchange client.

        Supports three modes:
        1. API mode: Set api_base_url and exchange_name to auto-fetch connection info
        2. Direct mode: Set nats_url and subjects for direct connection
        3. Hybrid mode: Combine API and custom configuration

        Args:
            config: Client configuration

        Returns:
            Initialized ExchangeClient instance

        Raises:
            Exception: If connection to NATS fails or API call fails
        """
        # If API is configured and we need connection info
        if config.api_base_url and config.exchange_name:
            if not config.nats_url or not config.subject_input or not config.subject_output:
                # Fetch connection info from API
                async with aiohttp.ClientSession() as session:
                    exchange_info = await cls._get_exchange(
                        session,
                        config.api_base_url,
                        config.exchange_namespace,
                        config.exchange_name,
                        config.api_key,
                    )

                    # Update config with connection info
                    if not config.nats_url:
                        config.nats_url = exchange_info["connectionInfo"]["natsUrl"]
                    if not config.subject_input:
                        config.subject_input = exchange_info["connectionInfo"]["inputSubject"]
                    if not config.subject_output:
                        config.subject_output = exchange_info["connectionInfo"]["outputSubject"]

        # Validate we have required NATS configuration
        if not config.nats_url or not config.subject_input or not config.subject_output:
            raise ValueError(
                "Either provide (nats_url, subject_input, subject_output) directly "
                "or (api_base_url, exchange_name) to fetch them from API"
            )

        # Connect to NATS
        nc = await nats.connect(
            servers=config.nats_url,
            max_reconnect_attempts=-1,
            reconnect_time_wait=2,
        )

        # Create JetStream context
        js = nc.jetstream()

        return cls(nc, js, config)

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

        await self.js.publish(self.config.subject_input, msg.to_bytes())

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

        sub = await self.nc.subscribe(self.config.subject_output, cb=message_handler)

        try:
            # Publish the request
            await self.js.publish(self.config.subject_input, msg.to_bytes())

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

            await client.subscribe(handle_response)
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

        sub = await self.nc.subscribe(self.config.subject_output, cb=message_handler)
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
            self.config.subject_output,
            queue=queue_group,
            cb=message_handler,
        )
        self._subscriptions.append(sub)

    async def delete_exchange(self) -> None:
        """
        Delete the Exchange via the API (requires API configuration).

        Raises:
            ValueError: If API is not configured
            Exception: If API call fails
        """
        if not self.config.api_base_url or not self.config.exchange_name:
            raise ValueError("API not configured or Exchange name not set")

        namespace = self.config.exchange_namespace or "default"
        url = f"{self.config.api_base_url}/api/v1/exchanges/{namespace}/{self.config.exchange_name}"

        headers = {}
        if self.config.api_key:
            headers["Authorization"] = f"Bearer {self.config.api_key}"

        async with aiohttp.ClientSession() as session:
            async with session.delete(url, headers=headers) as resp:
                if resp.status != 204:
                    text = await resp.text()
                    raise Exception(f"API error (status {resp.status}): {text}")

    async def close(self) -> None:
        """Close the NATS connection and clean up resources."""
        # Unsubscribe from all subscriptions
        for sub in self._subscriptions:
            await sub.unsubscribe()

        # Close HTTP session if exists
        if self._http_session:
            await self._http_session.close()

        # Close NATS connection
        if self.nc:
            await self.nc.close()

    @staticmethod
    async def _create_exchange(
        session: aiohttp.ClientSession,
        api_base_url: str,
        req: ExchangeRequest,
        api_key: Optional[str] = None,
    ) -> dict:
        """Create an Exchange via the API."""
        url = f"{api_base_url}/api/v1/exchanges"

        headers = {"Content-Type": "application/json"}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        body = {
            "name": req.name,
            "namespace": req.namespace,
            "image": req.image,
        }
        if req.env:
            body["env"] = req.env

        async with session.post(url, json=body, headers=headers) as resp:
            if resp.status != 201:
                text = await resp.text()
                raise Exception(f"Failed to create Exchange (status {resp.status}): {text}")
            return await resp.json()

    @staticmethod
    async def _get_exchange(
        session: aiohttp.ClientSession,
        api_base_url: str,
        namespace: str,
        name: str,
        api_key: Optional[str] = None,
    ) -> dict:
        """Get Exchange information from the API."""
        url = f"{api_base_url}/api/v1/exchanges/{namespace}/{name}"

        headers = {}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        async with session.get(url, headers=headers) as resp:
            if resp.status != 200:
                text = await resp.text()
                raise Exception(f"Failed to get Exchange (status {resp.status}): {text}")
            return await resp.json()

    @staticmethod
    async def _wait_for_ready(
        session: aiohttp.ClientSession,
        api_base_url: str,
        namespace: str,
        name: str,
        timeout: float = 60.0,
        api_key: Optional[str] = None,
    ) -> dict:
        """Wait for Exchange to become ready."""
        import time

        start = time.time()
        while time.time() - start < timeout:
            exchange = await ExchangeClient._get_exchange(session, api_base_url, namespace, name, api_key)

            if exchange.get("status", {}).get("phase") == "Running":
                return exchange

            await asyncio.sleep(2)

        raise TimeoutError(f"Exchange {namespace}/{name} did not become ready within {timeout}s")


async def create_exchange(
    api_base_url: str,
    req: ExchangeRequest,
    api_key: Optional[str] = None,
) -> ExchangeClient:
    """
    Create a new Exchange via the API and return a fully connected client.

    This is the easiest way to get started with Conduit.

    Args:
        api_base_url: Conduit API server URL (e.g., "http://localhost:8090")
        req: Exchange configuration (name, namespace, image, env vars, etc.)
        api_key: Optional API key for authentication

    Returns:
        A connected ExchangeClient ready to send/receive messages

    Example:
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

        # Send a message
        await client.send({"message": "Hello!"})

        # Clean up
        await client.close()
        await client.delete_exchange()
        ```
    """
    async with aiohttp.ClientSession() as session:
        # Create the Exchange
        exchange = await ExchangeClient._create_exchange(session, api_base_url, req, api_key)

        # Wait for it to be ready
        exchange = await ExchangeClient._wait_for_ready(
            session, api_base_url, req.namespace, req.name, 60.0, api_key
        )

        # Create a full client connected to the Exchange
        return await ExchangeClient.create(
            ClientConfig(
                api_base_url=api_base_url,
                api_key=api_key,
                nats_url=exchange["connectionInfo"]["natsUrl"],
                subject_input=exchange["connectionInfo"]["inputSubject"],
                subject_output=exchange["connectionInfo"]["outputSubject"],
                exchange_name=req.name,
                exchange_namespace=req.namespace,
            )
        )
