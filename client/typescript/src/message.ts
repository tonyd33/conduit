/**
 * Message envelope format for Conduit.
 */

import { v4 as uuidv4 } from 'uuid';

/**
 * Message type enumeration.
 */
export enum MessageType {
  DATA = 'data',
  CONTROL = 'control',
  ERROR = 'error',
}

/**
 * Message envelope for Conduit exchanges.
 */
export interface Message<T = any> {
  /** Unique message identifier */
  id: string;

  /** ISO8601 timestamp when message was created */
  timestamp: string;

  /** NATS stream sequence number */
  sequence: number;

  /** Message type */
  type: MessageType;

  /** Message content (arbitrary JSON-serializable data) */
  payload: T;

  /** Optional metadata */
  metadata?: Record<string, string>;
}

/**
 * Create a new message with the given type and payload.
 */
export function createMessage<T>(
  type: MessageType,
  payload: T,
  metadata?: Record<string, string>
): Message<T> {
  return {
    id: uuidv4(),
    timestamp: new Date().toISOString(),
    sequence: 0,
    type,
    payload,
    metadata: metadata || {},
  };
}

/**
 * Create a new data message (convenience function).
 */
export function createDataMessage<T>(
  payload: T,
  metadata?: Record<string, string>
): Message<T> {
  return createMessage(MessageType.DATA, payload, metadata);
}

/**
 * Serialize a message to JSON bytes.
 */
export function serializeMessage<T>(msg: Message<T>): Uint8Array {
  const json = JSON.stringify(msg);
  return new TextEncoder().encode(json);
}

/**
 * Deserialize a message from JSON bytes.
 */
export function parseMessage<T>(data: Uint8Array): Message<T> {
  const json = new TextDecoder().decode(data);
  const msg = JSON.parse(json) as Message<T>;
  validateMessage(msg);
  return msg;
}

/**
 * Validate that a message has all required fields.
 *
 * @throws Error if message is invalid
 */
export function validateMessage<T>(msg: Message<T>): void {
  if (!msg.id) {
    throw new Error('message ID is required');
  }
  if (!msg.type) {
    throw new Error('message type is required');
  }
  if (
    msg.type !== MessageType.DATA &&
    msg.type !== MessageType.CONTROL &&
    msg.type !== MessageType.ERROR
  ) {
    throw new Error(`invalid message type: ${msg.type}`);
  }
  if (msg.payload === undefined || msg.payload === null) {
    throw new Error('message payload is required');
  }
}
