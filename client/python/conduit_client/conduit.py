"""Conduit class for managing Exchange lifecycle via the API."""

import asyncio
from typing import Optional

import aiohttp
import nats
from nats.js import JetStreamContext

from .exchange import ExchangeClient
from .types import ExchangeRequest, ExchangeResource


class Conduit:
    """
    Conduit manages Exchange lifecycle via the API.

    This class handles creating, waiting for readiness, and deleting Exchanges
    via the Conduit API server.

    Example:
        ```python
        conduit = Conduit(api_base_url="http://localhost:8090")

        # Create Exchange and get connected client
        exchange = await conduit.create_exchange_client(
            ExchangeRequest(
                name="my-exchange",
                namespace="default",
                image="worker:latest",
            )
        )

        # Use exchange client...
        await exchange.send({"message": "Hello!"})

        # Clean up
        await exchange.close()
        await conduit.delete_exchange_client(exchange)
        ```
    """

    def __init__(self, api_base_url: str, api_key: Optional[str] = None):
        """
        Initialize Conduit API client.

        Args:
            api_base_url: Conduit API server URL (e.g., "http://localhost:8090")
            api_key: Optional API key for authentication
        """
        self.api_base_url = api_base_url
        self.api_key = api_key

    async def create_exchange_client(self, req: ExchangeRequest) -> ExchangeClient:
        """
        Create a new Exchange via the API and return a connected client.

        This is the easiest way to get started with Conduit.

        Args:
            req: Exchange configuration (name, namespace, image, env vars, etc.)

        Returns:
            A connected ExchangeClient ready to send/receive messages

        Example:
            ```python
            exchange = await conduit.create_exchange_client(
                ExchangeRequest(
                    name="my-exchange",
                    namespace="default",
                    image="worker:latest",
                    env={"KEY": "value"},
                )
            )
            ```
        """
        async with aiohttp.ClientSession() as session:
            # Create the Exchange
            resource = await self._create_exchange(session, req)

            # Wait for it to be ready
            resource = await self._wait_for_exchange_ready(
                session,
                resource["name"],
                resource["namespace"],
            )

        # Connect to NATS
        nc = await nats.connect(
            servers=resource["connectionInfo"]["natsURL"],
            max_reconnect_attempts=-1,
            reconnect_time_wait=2,
        )

        # Create JetStream context
        js = nc.jetstream()

        return ExchangeClient(
            nc=nc,
            js=js,
            subject_input=resource["connectionInfo"]["inputSubject"],
            subject_output=resource["connectionInfo"]["outputSubject"],
            name=resource["name"],
            namespace=resource["namespace"],
        )

    async def delete_exchange_client(self, exchange: ExchangeClient) -> None:
        """
        Delete an Exchange via the API.

        Args:
            exchange: The ExchangeClient instance to delete

        Example:
            ```python
            await conduit.delete_exchange_client(exchange)
            ```
        """
        await self._delete_exchange(exchange.name, exchange.namespace)

    async def _create_exchange(
        self,
        session: aiohttp.ClientSession,
        req: ExchangeRequest,
    ) -> dict:
        """Create an Exchange via the API (private)."""
        url = f"{self.api_base_url}/api/v1/exchanges"

        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

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

    async def _get_exchange(
        self,
        session: aiohttp.ClientSession,
        name: str,
        namespace: str,
    ) -> dict:
        """Get Exchange information from the API (private)."""
        url = f"{self.api_base_url}/api/v1/exchanges/{namespace}/{name}"

        headers = {}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        async with session.get(url, headers=headers) as resp:
            if resp.status != 200:
                text = await resp.text()
                raise Exception(f"Failed to get Exchange (status {resp.status}): {text}")
            return await resp.json()

    async def _wait_for_exchange_ready(
        self,
        session: aiohttp.ClientSession,
        name: str,
        namespace: str,
        timeout: float = 60.0,
    ) -> dict:
        """Wait for Exchange to become ready (private)."""
        import time

        start = time.time()
        while time.time() - start < timeout:
            resource = await self._get_exchange(session, name, namespace)

            if resource.get("phase") == "Running":
                return resource

            await asyncio.sleep(2)

        raise TimeoutError(f"Exchange {namespace}/{name} did not become ready within {timeout}s")

    async def _delete_exchange(self, name: str, namespace: str) -> None:
        """Delete an Exchange via the API (private)."""
        url = f"{self.api_base_url}/api/v1/exchanges/{namespace}/{name}"

        headers = {}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        async with aiohttp.ClientSession() as session:
            async with session.delete(url, headers=headers) as resp:
                if resp.status != 204:
                    text = await resp.text()
                    raise Exception(f"API error (status {resp.status}): {text}")
