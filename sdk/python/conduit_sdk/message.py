"""
Message envelope types for Conduit SDK.

Defines the standard message format used for communication.
"""

import json
from datetime import datetime
from enum import Enum
from typing import Any, Dict, Optional
from dataclasses import dataclass, field
from uuid import uuid4


class MessageType(str, Enum):
    """Type of message."""

    DATA = "data"
    CONTROL = "control"
    ERROR = "error"


@dataclass
class Message:
    """
    Message envelope for Conduit communication.

    Attributes:
        id: Unique message identifier
        timestamp: When the message was created (ISO 8601 format)
        sequence: NATS sequence number
        type: Message type (data, control, or error)
        payload: The actual message payload (any JSON-serializable value)
        metadata: Optional metadata dictionary
    """

    id: str
    timestamp: str
    sequence: int
    type: MessageType
    payload: Any
    metadata: Optional[Dict[str, str]] = None

    @classmethod
    def create(
        cls,
        msg_type: MessageType,
        payload: Any,
        metadata: Optional[Dict[str, str]] = None,
    ) -> "Message":
        """
        Create a new message.

        Args:
            msg_type: Type of message
            payload: Message payload (must be JSON-serializable)
            metadata: Optional metadata dictionary

        Returns:
            Message: New message instance
        """
        return cls(
            id=str(uuid4()),
            timestamp=datetime.utcnow().isoformat() + "Z",
            sequence=0,  # Will be set by NATS
            type=msg_type,
            payload=payload,
            metadata=metadata or {},
        )

    @classmethod
    def from_json(cls, data: bytes) -> "Message":
        """
        Parse a message from JSON bytes.

        Args:
            data: JSON-encoded message

        Returns:
            Message: Parsed message instance

        Raises:
            ValueError: If the message cannot be parsed
        """
        try:
            parsed = json.loads(data)
            return cls(
                id=parsed["id"],
                timestamp=parsed["timestamp"],
                sequence=parsed.get("sequence", 0),
                type=MessageType(parsed["type"]),
                payload=parsed["payload"],
                metadata=parsed.get("metadata"),
            )
        except (json.JSONDecodeError, KeyError, ValueError) as e:
            raise ValueError(f"Failed to parse message: {e}") from e

    def to_json(self) -> bytes:
        """
        Serialize message to JSON bytes.

        Returns:
            bytes: JSON-encoded message
        """
        data = {
            "id": self.id,
            "timestamp": self.timestamp,
            "sequence": self.sequence,
            "type": self.type.value,
            "payload": self.payload,
        }
        if self.metadata:
            data["metadata"] = self.metadata

        return json.dumps(data).encode("utf-8")

    def unmarshal_payload(self, target_type: type) -> Any:
        """
        Unmarshal the payload into a specific type.

        For dict/list payloads, this returns the payload as-is.
        For other types, attempts to construct the type from the payload.

        Args:
            target_type: The type to unmarshal into

        Returns:
            The unmarshaled payload

        Raises:
            ValueError: If unmarshaling fails
        """
        if isinstance(self.payload, target_type):
            return self.payload

        if target_type in (dict, list, str, int, float, bool):
            if isinstance(self.payload, target_type):
                return self.payload
            raise ValueError(
                f"Payload type {type(self.payload)} does not match target type {target_type}"
            )

        # For custom types, try to construct from dict
        if isinstance(self.payload, dict):
            try:
                return target_type(**self.payload)
            except TypeError as e:
                raise ValueError(f"Cannot construct {target_type} from payload: {e}") from e

        raise ValueError(f"Cannot unmarshal payload of type {type(self.payload)} into {target_type}")

    def __repr__(self) -> str:
        """String representation."""
        return (
            f"Message(id={self.id[:8]}..., type={self.type.value}, "
            f"sequence={self.sequence}, payload_type={type(self.payload).__name__})"
        )
