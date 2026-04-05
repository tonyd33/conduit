/**
 * Main Conduit SDK client.
 *
 * Provides async interface for message processing and publishing.
 */

import {
  connect,
  NatsConnection,
  JetStreamClient,
} from 'nats';
import { Config, loadConfigFromEnv } from './config';
import { Message, MessageType, createMessage, parseMessage, serializeMessage } from './message';

/**
 * Message handler function type.
 */
export type MessageHandler<T = any> = (msg: Message<T>) => Promise<void>;

/**
 * Conduit SDK client for Exchange-compatible applications.
 *
 * Provides async interface for:
 * - Receiving messages from NATS JetStream
 * - Publishing messages to output subject
 * - Handling control messages
 *
 * @example
 * ```typescript
 * import { Client, Message } from 'conduit-sdk';
 *
 * const client = await Client.create();
 *
 * await client.run(async (msg: Message) => {
 *   console.log('Received:', msg.payload);
 *   await client.publish({ response: 'processed' });
 * });
 *
 * await client.close();
 * ```
 */
export class Client {
  private config: Config;
  private nc: NatsConnection;
  private js: JetStreamClient;
  private paused: boolean = false;
  private subscription?: any;

  private constructor(
    config: Config,
    nc: NatsConnection,
    js: JetStreamClient
  ) {
    this.config = config;
    this.nc = nc;
    this.js = js;
  }

  /**
   * Create a new Conduit client.
   *
   * @param config Configuration object (loads from env if not provided)
   * @returns Initialized Client instance
   * @throws Error if connection to NATS fails
   */
  static async create(config?: Config): Promise<Client> {
    if (!config) {
      config = loadConfigFromEnv();
    }

    // console.log(`Connecting to NATS at ${config.natsUrl}`);

    // Connect to NATS
    const nc = await connect({
      servers: config.natsUrl,
    });

    const js = nc.jetstream();

    const client = new Client(config, nc, js);

    return client;
  }

  /**
   * Close the NATS connection.
   */
  async close(): Promise<void> {
    if (this.subscription) {
      await this.subscription.unsubscribe();
    }
    await this.nc.close();
    // console.log('Client closed');
  }

  /**
   * Publish a message to the output subject.
   *
   * @param payload Message payload (must be JSON-serializable)
   * @param metadata Optional metadata
   * @throws Error if publishing fails
   */
  async publish<T>(payload: T, metadata?: Record<string, string>): Promise<void> {
    const msg = createMessage(MessageType.DATA, payload, metadata);
    const data = serializeMessage(msg);

    await this.js.publish(this.config.subjectOutput, data);
    // console.debug(`Published message to ${this.config.subjectOutput}`);
  }

  /**
   * Handle control messages from the operator.
   *
   * @param msg Control message
   */
  private async handleControlMessage(msg: Message): Promise<void> {
    const payload = msg.payload;
    if (typeof payload === 'object' && payload !== null) {
      const command = (payload as any).command;
      if (command === 'pause') {
        this.paused = true;
        // console.log('Application paused by control message');
      } else if (command === 'resume') {
        this.paused = false;
        // console.log('Application resumed by control message');
      }
    }
  }

  /**
   * Start processing messages with the provided handler.
   *
   * This method blocks until the client is closed or an error occurs.
   *
   * @param handler Async function that processes messages
   * @throws Error if message processing fails
   */
  async run<T = any>(handler: MessageHandler<T>): Promise<void> {
    // console.log(`Starting message processing on ${this.config.subjectInput}`);

    // Create consumer
    const consumer = await this.js.consumers.get(this.config.streamName, this.config.consumerName);

    // Subscribe to messages
    const messages = await consumer.consume({
      max_messages: 0, // Infinite
    });

    // console.log('Ready to process messages');

    try {
      for await (const natsMsg of messages) {
        try {
          // Parse the message
          const msg = parseMessage<T>(natsMsg.data);
          msg.sequence = natsMsg.seq;

          // Handle control messages
          if (msg.type === MessageType.CONTROL) {
            await this.handleControlMessage(msg);
            natsMsg.ack();
            continue;
          }

          // Skip processing if paused
          if (this.paused) {
            // console.debug(`Skipping message ${msg.sequence} (paused)`);
            continue;
          }

          // Call user handler
          await handler(msg);

          // Acknowledge the message
          natsMsg.ack();
          // console.debug(`Processed message sequence ${msg.sequence}`);
        } catch (error) {
          // console.error('Error processing message:', error);
          // NAK the message so it can be redelivered
          natsMsg.nak();
        }
      }
    } catch (error) {
      if ((error as any).message?.includes('cancelled')) {
        // console.log('Message processing cancelled');
      } else {
        throw error;
      }
    }
  }
}
