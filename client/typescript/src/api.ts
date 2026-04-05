export interface ConnectionInfo {
  natsURL: string
  streamName: string
  inputSubject: string
  outputSubject: string
}

type EnvVar = { Name: string, Value: string };

/**
 * Request to create a new Exchange via the API.
 */
export interface ExchangeRequest {
  /** Exchange name */
  name: string;

  /** Container image for the Exchange worker */
  image: string;

  /** Kubernetes namespace (defaults to "default") */
  namespace?: string;

  /** Optional environment variables */
  env?: EnvVar[]
}

export interface ExchangeResource {
  name: string
  namespace: string
  uid: string
  phase: string
  message?: string
  connectionInfo: ConnectionInfo
}
