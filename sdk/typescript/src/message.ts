/**
 * Message envelope types for Conduit SDK.
 *
 * Defines the standard message format used for communication.
 */

import { v4 as uuidv4 } from 'uuid';

/**
 * Type of message.
 */
export enum MessageType {
  DATA = 'data',
  CONTROL = 'control',
  ERROR = 'error',
}

/**
 * Message envelope for Conduit communication.
 */
export interface Message<T = any> {
  /** Unique message identifier */
  id: string;
  /** When the message was created (ISO 8601 format) */
  timestamp: string;
  /** NATS sequence number */
  sequence: number;
  /** Message type (data, control, or error) */
  type: MessageType;
  /** The actual message payload */
  payload: T;
  /** Optional metadata */
  metadata?: Record<string, string>;
}

/**
 * Create a new message.
 *
 * @param type Message type
 * @param payload Message payload (must be JSON-serializable)
 * @param metadata Optional metadata
 * @returns New message instance
 */
export function createMessage<T>(
  type: MessageType,
  payload: T,
  metadata?: Record<string, string>
): Message<T> {
  return {
    id: uuidv4(),
    timestamp: new Date().toISOString(),
    sequence: 0, // Will be set by NATS
    type,
    payload,
    metadata,
  };
}

/**
 * Parse a message from JSON bytes.
 *
 * @param data JSON-encoded message
 * @returns Parsed message instance
 * @throws Error if the message cannot be parsed
 */
export function parseMessage<T = any>(data: Uint8Array | string): Message<T> {
  try {
    const text = typeof data === 'string' ? data : new TextDecoder().decode(data);
    const parsed = JSON.parse(text);

    if (!parsed.id || !parsed.timestamp || !parsed.type) {
      throw new Error('Missing required message fields');
    }

    return {
      id: parsed.id,
      timestamp: parsed.timestamp,
      sequence: parsed.sequence || 0,
      type: parsed.type as MessageType,
      payload: parsed.payload,
      metadata: parsed.metadata,
    };
  } catch (error) {
    throw new Error(`Failed to parse message: ${error}`);
  }
}

/**
 * Serialize message to JSON bytes.
 *
 * @param msg Message to serialize
 * @returns JSON-encoded message
 */
export function serializeMessage<T>(msg: Message<T>): Uint8Array {
  const data: any = {
    id: msg.id,
    timestamp: msg.timestamp,
    sequence: msg.sequence,
    type: msg.type,
    payload: msg.payload,
  };

  if (msg.metadata) {
    data.metadata = msg.metadata;
  }

  return new TextEncoder().encode(JSON.stringify(data));
}
