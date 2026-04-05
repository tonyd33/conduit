"""Message envelope format for Conduit."""

import json
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Dict, Optional


class MessageType(str, Enum):
    """Message type enumeration."""

    DATA = "data"
    CONTROL = "control"
    ERROR = "error"


@dataclass
class Message:
    """
    Message envelope for Conduit exchanges.

    Attributes:
        id: Unique message identifier
        timestamp: ISO8601 timestamp when message was created
        sequence: NATS stream sequence number
        type: Message type (data, control, error)
        payload: Message content (arbitrary JSON-serializable data)
        metadata: Optional metadata dictionary
    """

    id: str
    timestamp: str
    sequence: int
    type: MessageType
    payload: Any
    metadata: Optional[Dict[str, str]] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, Any]:
        """Convert message to dictionary."""
        return {
            "id": self.id,
            "timestamp": self.timestamp,
            "sequence": self.sequence,
            "type": self.type.value,
            "payload": self.payload,
            "metadata": self.metadata or {},
        }

    def to_json(self) -> str:
        """Serialize message to JSON string."""
        return json.dumps(self.to_dict())

    def to_bytes(self) -> bytes:
        """Serialize message to JSON bytes."""
        return self.to_json().encode("utf-8")

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Message":
        """Create message from dictionary."""
        return cls(
            id=data["id"],
            timestamp=data["timestamp"],
            sequence=data.get("sequence", 0),
            type=MessageType(data["type"]),
            payload=data["payload"],
            metadata=data.get("metadata"),
        )

    @classmethod
    def from_json(cls, data: str) -> "Message":
        """Deserialize message from JSON string."""
        return cls.from_dict(json.loads(data))

    @classmethod
    def from_bytes(cls, data: bytes) -> "Message":
        """Deserialize message from JSON bytes."""
        return cls.from_json(data.decode("utf-8"))


def new_message(msg_type: MessageType, payload: Any, metadata: Optional[Dict[str, str]] = None) -> Message:
    """
    Create a new message with the given type and payload.

    Args:
        msg_type: Message type
        payload: Message payload (must be JSON-serializable)
        metadata: Optional metadata dictionary

    Returns:
        New Message instance
    """
    return Message(
        id=str(uuid.uuid4()),
        timestamp=datetime.utcnow().isoformat() + "Z",
        sequence=0,
        type=msg_type,
        payload=payload,
        metadata=metadata or {},
    )


def new_data_message(payload: Any, metadata: Optional[Dict[str, str]] = None) -> Message:
    """
    Create a new data message (convenience function).

    Args:
        payload: Message payload (must be JSON-serializable)
        metadata: Optional metadata dictionary

    Returns:
        New Message instance with type=DATA
    """
    return new_message(MessageType.DATA, payload, metadata)


def validate_message(msg: Message) -> None:
    """
    Validate that a message has all required fields.

    Args:
        msg: Message to validate

    Raises:
        ValueError: If message is invalid
    """
    if not msg.id:
        raise ValueError("message ID is required")
    if not msg.type:
        raise ValueError("message type is required")
    if msg.type not in (MessageType.DATA, MessageType.CONTROL, MessageType.ERROR):
        raise ValueError(f"invalid message type: {msg.type}")
    if msg.payload is None:
        raise ValueError("message payload is required")
