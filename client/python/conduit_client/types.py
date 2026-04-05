"""Type definitions for Conduit API."""

from dataclasses import dataclass
from typing import Optional


@dataclass
class ExchangeRequest:
    """
    Request to create a new Exchange via the API.

    Attributes:
        name: Exchange name
        image: Container image for the Exchange worker
        namespace: Kubernetes namespace (defaults to "default")
        env: Optional environment variables
    """

    name: str
    image: str
    namespace: str = "default"
    env: Optional[dict[str, str]] = None


@dataclass
class ConnectionInfo:
    """
    NATS connection information for an Exchange.

    Attributes:
        nats_url: NATS server URL
        stream_name: JetStream stream name
        input_subject: Subject for sending messages to Exchange
        output_subject: Subject for receiving messages from Exchange
    """

    nats_url: str
    stream_name: str
    input_subject: str
    output_subject: str


@dataclass
class ExchangeResource:
    """
    Exchange resource from the API.

    Attributes:
        name: Exchange name
        namespace: Exchange namespace
        uid: Exchange UID
        phase: Current phase (e.g., "Running", "Pending")
        message: Optional status message
        connection_info: NATS connection details
    """

    name: str
    namespace: str
    uid: str
    phase: str
    connection_info: ConnectionInfo
    message: Optional[str] = None
