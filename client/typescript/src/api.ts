export interface ConnectionInfo {
  natsURL: string
  streamName: string
  inputSubject: string
  outputSubject: string
}

/**
 * Environment variable for a container.
 */
export interface EnvVar {
  /** Name of the environment variable */
  name: string;
  /** Value of the environment variable */
  value: string;
}

/**
 * Volume mount configuration.
 */
export interface VolumeMount {
  /** Name of the volume to mount */
  name: string;
  /** Path within the container at which the volume should be mounted */
  mountPath: string;
  /** Mounted read-only if true, read-write otherwise (default false) */
  readOnly?: boolean;
}

/**
 * Volume that can be mounted by containers.
 * Simplified to support common volume types.
 */
export interface Volume {
  /** Name of the volume - must be unique within a pod */
  name: string;
  /** EmptyDir represents a temporary directory that shares a pod's lifetime */
  emptyDir?: Record<string, never>;
  /** ConfigMap represents a configMap that should populate this volume */
  configMap?: {
    name: string;
  };
  /** Secret represents a secret that should populate this volume */
  secret?: {
    secretName: string;
  };
  /** PersistentVolumeClaim represents a reference to a PersistentVolumeClaim */
  persistentVolumeClaim?: {
    claimName: string;
  };
}

/**
 * Request to create a new Exchange.
 */
export interface ExchangeRequest {
  /** Exchange name */
  name: string;

  /** Container image for the Exchange worker */
  image: string;

  /** Kubernetes namespace (defaults to "default") */
  namespace?: string;

  /** Optional environment variables */
  env?: EnvVar[];

  /** Optional volume mounts */
  volumeMounts?: VolumeMount[];

  /** Optional volumes */
  volumes?: Volume[];
}

export interface ExchangeResource {
  name: string
  namespace: string
  uid: string
  phase: string
  message?: string
  connectionInfo: ConnectionInfo
}
