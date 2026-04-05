/**
 * Configuration loader for Conduit SDK.
 *
 * Loads configuration from environment variables injected by the Conduit operator.
 */

/**
 * Configuration for the Conduit client.
 *
 * All values are loaded from environment variables injected by the operator.
 */
export interface Config {
  /** Unique identifier for this Exchange instance */
  exchangeId: string;
  /** Name of the Exchange resource */
  exchangeName: string;
  /** Kubernetes namespace */
  exchangeNamespace: string;
  /** NATS server URL */
  natsUrl: string;
  /** JetStream stream name */
  streamName: string;
  /** Durable consumer name */
  consumerName: string;
  /** Subject to receive messages from */
  subjectInput: string;
  /** Subject to publish messages to */
  subjectOutput: string;
  /** Subject for control messages */
  subjectControl: string;
}

/**
 * Load configuration from environment variables.
 *
 * @returns Configuration object populated from environment
 * @throws Error if required environment variables are missing
 */
export function loadConfigFromEnv(): Config {
  // Required fields
  const requiredFields = {
    EXCHANGE_ID: 'exchangeId',
    EXCHANGE_NAME: 'exchangeName',
    EXCHANGE_NAMESPACE: 'exchangeNamespace',
    NATS_URL: 'natsUrl',
    NATS_STREAM_NAME: 'streamName',
    NATS_CONSUMER_NAME: 'consumerName',
    NATS_SUBJECT_INPUT: 'subjectInput',
    NATS_SUBJECT_OUTPUT: 'subjectOutput',
    NATS_SUBJECT_CONTROL: 'subjectControl',
  } as const;

  const config: Partial<Config> = {};
  const missing: string[] = [];

  // Load required fields
  for (const [envVar, fieldName] of Object.entries(requiredFields)) {
    const value = process.env[envVar];
    if (value === undefined) {
      missing.push(envVar);
    } else {
      (config as any)[fieldName] = value;
    }
  }

  if (missing.length > 0) {
    throw new Error(
      `Missing required environment variables: ${missing.join(', ')}`
    );
  }

  return config as Config;
}
