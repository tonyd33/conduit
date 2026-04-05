"""
Configuration loader for Conduit SDK.

Loads configuration from environment variables injected by the Conduit operator.
"""

import os
from typing import Optional
from dataclasses import dataclass


@dataclass
class Config:
    """
    Configuration for the Conduit client.

    All values are loaded from environment variables injected by the operator.
    """

    exchange_id: str
    exchange_name: str
    exchange_namespace: str
    nats_url: str
    stream_name: str
    consumer_name: str
    subject_input: str
    subject_output: str
    subject_control: str

    @classmethod
    def from_env(cls) -> "Config":
        """
        Load configuration from environment variables.

        Returns:
            Config: Configuration object populated from environment

        Raises:
            ValueError: If required environment variables are missing
        """
        # Required fields
        required_fields = {
            "EXCHANGE_ID": "exchange_id",
            "EXCHANGE_NAME": "exchange_name",
            "EXCHANGE_NAMESPACE": "exchange_namespace",
            "NATS_URL": "nats_url",
            "NATS_STREAM_NAME": "stream_name",
            "NATS_CONSUMER_NAME": "consumer_name",
            "NATS_SUBJECT_INPUT": "subject_input",
            "NATS_SUBJECT_OUTPUT": "subject_output",
            "NATS_SUBJECT_CONTROL": "subject_control",
        }

        config_dict = {}
        missing = []

        for env_var, field_name in required_fields.items():
            value = os.getenv(env_var)
            if value is None:
                missing.append(env_var)
            else:
                config_dict[field_name] = value

        if missing:
            raise ValueError(
                f"Missing required environment variables: {', '.join(missing)}"
            )

        return cls(**config_dict)

    def __repr__(self) -> str:
        """String representation with sensitive data masked."""
        return (
            f"Config(exchange_id={self.exchange_id}, "
            f"exchange_name={self.exchange_name}, "
            f"namespace={self.exchange_namespace}, "
            f"nats_url={self.nats_url}, "
            f"stream={self.stream_name})"
        )
