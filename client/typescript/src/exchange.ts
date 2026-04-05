import { NatsConnection, JetStreamClient, Msg } from 'nats';
import {
  Message,
  createDataMessage,
  serializeMessage,
  parseMessage,
  validateMessage,
} from './message';

/**
 * Response handler function type.
 */
export type ResponseHandler<T = any> = (msg: Message<T>) => Promise<void> | void;

export class ExchangeClient {
  private nc: NatsConnection;
  private js: JetStreamClient;
  private subjectInput: string;
  private subjectOutput: string;
  name: string;
  namespace: string;

  constructor(
    nc: NatsConnection,
    js: JetStreamClient,
    subjectInput: string,
    subjectOutput: string,
    name: string,
    namespace: string
  ) {
    this.nc = nc;
    this.js = js;
    this.subjectInput = subjectInput;
    this.subjectOutput = subjectOutput;
    this.name = name;
    this.namespace = namespace;
  }

  /**
   * Send a message to the Exchange without waiting for a response.
   *
   * @param payload Message payload (must be JSON-serializable)
   * @param metadata Optional metadata dictionary
   * @throws Error if publishing fails
   */
  async send<T>(payload: T, metadata?: Record<string, string>): Promise<void> {
    const msg = createDataMessage(payload, metadata);
    validateMessage(msg);

    const data = serializeMessage(msg);
    await this.js.publish(this.subjectInput, data);
  }

  /**
   * Subscribe to all responses from the Exchange.
   *
   * @param handler Function that processes response messages
   * @example
   * ```typescript
   * await client.subscribe(async (msg) => {
   *   console.log('Received:', msg.payload);
   * });
   * ```
   */
  async subscribe<T>(handler: ResponseHandler<T>): Promise<void> {
    const sub = this.nc.subscribe(this.subjectOutput);

    for await (const natsMsg of sub) {
      try {
        const msg = parseMessage<T>(natsMsg.data);
        validateMessage(msg);
        await handler(msg);
      } catch (err) {
        // Silently ignore errors in handler
      }
    }
  }

  /**
   * Subscribe to responses using a NATS queue group.
   *
   * This allows multiple clients to load-balance response processing.
   *
   * @param queueGroup Name of the queue group
   * @param handler Function that processes response messages
   */
  async subscribeWithQueue<T>(
    queueGroup: string,
    handler: ResponseHandler<T>
  ): Promise<void> {
    const sub = this.nc.subscribe(this.subjectOutput, {
      queue: queueGroup,
    });

    for await (const natsMsg of sub) {
      try {
        const msg = parseMessage<T>(natsMsg.data);
        validateMessage(msg);
        await handler(msg);
      } catch (err) {
        // Silently ignore errors
      }
    }
  }

  /**
   * Close the NATS connection and clean up resources.
   */
  async close(): Promise<void> {
    if (this.nc) {
      await this.nc.close();
    }
  }
}
