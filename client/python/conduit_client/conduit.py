"""Conduit class for managing Exchange lifecycle via Kubernetes API."""

import asyncio
from typing import Optional

import nats
from kubernetes import client, config

from .exchange import ExchangeClient
from .types import ExchangeRequest, ExchangeResource


class Conduit:
    """
    Conduit manages Exchange lifecycle via Kubernetes API.

    This class handles creating, waiting for readiness, and deleting Exchanges
    by directly interacting with Kubernetes to create Exchange CRs.

    Example:
        ```python
        conduit = Conduit(
            namespace="default",
            nats_url="nats://conduit-nats.conduit-system.svc.cluster.local:4222"
        )

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

    def __init__(self, nats_url: str, namespace: str = "default"):
        """
        Initialize Conduit Kubernetes client.

        Args:
            nats_url: NATS server URL (e.g., "nats://conduit-nats.conduit-system.svc.cluster.local:4222")
            namespace: Default Kubernetes namespace for Exchanges
        """
        config.load_kube_config()
        self.k8s_api = client.CustomObjectsApi()
        self.namespace = namespace
        self.nats_url = nats_url

    async def create_exchange_client(self, req: ExchangeRequest) -> ExchangeClient:
        """
        Create a new Exchange via Kubernetes API and return a connected client.

        This is the easiest way to get started with Conduit.

        Args:
            req: Exchange configuration (name, namespace, image, env vars, etc.)

        Returns:
            A connected ExchangeClient ready to send/receive messages

        Example:
            ```python
            from conduit_client.types import EnvVar, VolumeMount, Volume

            exchange = await conduit.create_exchange_client(
                ExchangeRequest(
                    name="my-exchange",
                    namespace="default",
                    image="worker:latest",
                    env=[EnvVar(name="KEY", value="value")],
                    volume_mounts=[VolumeMount(name="data", mount_path="/data")],
                    volumes=[Volume(name="data", empty_dir={})],
                )
            )
            ```
        """
        # Create the Exchange CR
        await asyncio.to_thread(self._create_exchange, req)

        # Wait for it to be ready
        resource = await self._wait_for_exchange_ready(
            req.name,
            req.namespace or self.namespace,
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
        Delete an Exchange via Kubernetes API.

        Args:
            exchange: The ExchangeClient instance to delete

        Example:
            ```python
            await conduit.delete_exchange_client(exchange)
            ```
        """
        await asyncio.to_thread(self._delete_exchange, exchange.name, exchange.namespace)

    def _create_exchange(self, req: ExchangeRequest) -> None:
        """Create an Exchange CR via Kubernetes API (private)."""
        namespace = req.namespace or self.namespace

        # Build the container spec
        container = {
            "name": "exchange",
            "image": req.image,
        }

        if req.env:
            container["env"] = [
                {"name": e.name, "value": e.value} for e in req.env
            ]

        if req.volume_mounts:
            container["volumeMounts"] = [
                {
                    "name": vm.name,
                    "mountPath": vm.mount_path,
                    **({"readOnly": vm.read_only} if vm.read_only is not None else {}),
                }
                for vm in req.volume_mounts
            ]

        # Build the pod spec
        pod_spec = {"containers": [container]}

        if req.volumes:
            pod_spec["volumes"] = []
            for vol in req.volumes:
                volume_dict = {"name": vol.name}
                if vol.empty_dir is not None:
                    volume_dict["emptyDir"] = vol.empty_dir
                elif vol.config_map is not None:
                    volume_dict["configMap"] = vol.config_map
                elif vol.secret is not None:
                    volume_dict["secret"] = vol.secret
                elif vol.persistent_volume_claim is not None:
                    volume_dict["persistentVolumeClaim"] = vol.persistent_volume_claim
                pod_spec["volumes"].append(volume_dict)

        # Build the Exchange CR
        exchange_cr = {
            "apiVersion": "conduit.mnke.org/v1alpha1",
            "kind": "Exchange",
            "metadata": {
                "name": req.name,
                "namespace": namespace,
            },
            "spec": {
                "template": {
                    "spec": pod_spec,
                },
            },
        }

        try:
            self.k8s_api.create_namespaced_custom_object(
                group="conduit.mnke.org",
                version="v1alpha1",
                namespace=namespace,
                plural="exchanges",
                body=exchange_cr,
            )
        except Exception as e:
            raise Exception(f"Failed to create Exchange: {str(e)}")

    def _get_exchange(self, name: str, namespace: str) -> dict:
        """Get Exchange CR from Kubernetes API (private)."""
        try:
            exchange = self.k8s_api.get_namespaced_custom_object(
                group="conduit.mnke.org",
                version="v1alpha1",
                namespace=namespace,
                plural="exchanges",
                name=name,
            )

            # Extract connection info from Exchange status
            uid = exchange.get("metadata", {}).get("uid", "")
            spec = exchange.get("spec", {})
            status = exchange.get("status", {})

            input_subject = spec.get("stream", {}).get("subjects", {}).get("input")
            if not input_subject:
                input_subject = f"exchange.{uid}.input"

            output_subject = spec.get("stream", {}).get("subjects", {}).get("output")
            if not output_subject:
                output_subject = f"exchange.{uid}.output"

            connection_info = {
                "natsURL": self.nats_url,
                "streamName": status.get("streamName", ""),
                "inputSubject": input_subject,
                "outputSubject": output_subject,
            }

            return {
                "name": exchange.get("metadata", {}).get("name"),
                "namespace": exchange.get("metadata", {}).get("namespace"),
                "uid": uid,
                "phase": status.get("phase", "Pending"),
                "message": status.get("message"),
                "connectionInfo": connection_info,
            }
        except Exception as e:
            raise Exception(f"Failed to get Exchange: {str(e)}")

    async def _wait_for_exchange_ready(
        self,
        name: str,
        namespace: str,
        timeout: float = 60.0,
    ) -> dict:
        """Wait for Exchange to become ready (private)."""
        import time

        start = time.time()
        while time.time() - start < timeout:
            resource = await asyncio.to_thread(self._get_exchange, name, namespace)

            if resource.get("phase") == "Running":
                return resource

            await asyncio.sleep(2)

        raise TimeoutError(f"Exchange {namespace}/{name} did not become ready within {timeout}s")

    def _delete_exchange(self, name: str, namespace: str) -> None:
        """Delete an Exchange CR via Kubernetes API (private)."""
        try:
            self.k8s_api.delete_namespaced_custom_object(
                group="conduit.mnke.org",
                version="v1alpha1",
                namespace=namespace,
                plural="exchanges",
                name=name,
            )
        except Exception as e:
            raise Exception(f"Failed to delete Exchange: {str(e)}")
