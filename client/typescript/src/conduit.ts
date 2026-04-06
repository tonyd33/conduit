/**
 * Conduit client for managing Exchanges.
 */

import { ExchangeRequest, ExchangeResource, ConnectionInfo } from './api';
import { ExchangeClient } from './exchange';
import { connect } from 'nats';
import * as k8s from '@kubernetes/client-node';

export class Conduit {
  private k8sApi: k8s.CustomObjectsApi;
  private namespace: string;
  private natsURL: string;

  constructor({ namespace = 'default', natsURL }: { namespace?: string; natsURL: string }) {
    const kc = new k8s.KubeConfig();
    kc.loadFromDefault();

    this.k8sApi = kc.makeApiClient(k8s.CustomObjectsApi);
    this.namespace = namespace;
    this.natsURL = natsURL;
  }

  /**
   * Create Exchange CR via Kubernetes API.
   * @private
   */
  private async createExchange(req: ExchangeRequest): Promise<void> {
    const namespace = req.namespace || this.namespace;

    // Build the container spec
    const container: k8s.V1Container = {
      name: 'exchange',
      image: req.image,
      imagePullPolicy: "Never",
    };

    if (req.env && req.env.length > 0) {
      container.env = req.env;
    }

    if (req.volumeMounts && req.volumeMounts.length > 0) {
      container.volumeMounts = req.volumeMounts;
    }

    // Build the pod spec
    const podSpec: k8s.V1PodSpec = {
      containers: [container],
    };

    if (req.volumes && req.volumes.length > 0) {
      podSpec.volumes = req.volumes;
    }

    // Build the Exchange CR
    const exchangeCR = {
      apiVersion: 'conduit.mnke.org/v1alpha1',
      kind: 'Exchange',
      metadata: {
        name: req.name,
        namespace: namespace,
      },
      spec: {
        template: {
          spec: podSpec,
        },
      },
    };

    try {
      await this.k8sApi.createNamespacedCustomObject(
        'conduit.mnke.org',
        'v1alpha1',
        namespace,
        'exchanges',
        exchangeCR
      );
    } catch (error: any) {
      throw new Error(`Failed to create Exchange: ${error.body?.message || error.message}`);
    }
  }

  /**
   * Get Exchange CR from Kubernetes API.
   * @private
   */
  private async getExchange(
    name: string,
    namespace: string
  ): Promise<ExchangeResource> {
    try {
      const response = await this.k8sApi.getNamespacedCustomObject(
        'conduit.mnke.org',
        'v1alpha1',
        namespace,
        'exchanges',
        name
      );

      const exchange = response.body as any;

      // Extract connection info from Exchange status
      const connectionInfo: ConnectionInfo = {
        natsURL: this.natsURL,
        streamName: exchange.status?.streamName || '',
        inputSubject: exchange.spec?.stream?.subjects?.input || `exchange.${exchange.metadata.uid}.input`,
        outputSubject: exchange.spec?.stream?.subjects?.output || `exchange.${exchange.metadata.uid}.output`,
      };

      return {
        name: exchange.metadata.name,
        namespace: exchange.metadata.namespace,
        uid: exchange.metadata.uid,
        phase: exchange.status?.phase || 'Pending',
        message: exchange.status?.message,
        connectionInfo: connectionInfo,
      };
    } catch (error: any) {
      throw new Error(`Failed to get Exchange: ${error.body?.message || error.message}`);
    }
  }

  /**
   * Wait for Exchange to become ready.
   * @private
   */
  private async waitForExchangeReady(
    name: string,
    namespace: string,
    timeout: number = 60000
  ): Promise<ExchangeResource> {
    const start = Date.now();

    while (Date.now() - start < timeout) {
      const exchange = await this.getExchange(name, namespace);

      if (exchange.phase === 'Running') {
        return exchange;
      }

      await new Promise((resolve) => setTimeout(resolve, 2000));
    }

    throw new Error(
      `Exchange ${namespace}/${name} did not become ready within ${timeout}ms`
    );
  }

  /**
   * Delete an Exchange CR via Kubernetes API.
   */
  private async deleteExchange(name: string, namespace: string): Promise<void> {
    try {
      await this.k8sApi.deleteNamespacedCustomObject(
        'conduit.mnke.org',
        'v1alpha1',
        namespace,
        'exchanges',
        name
      );
    } catch (error: any) {
      throw new Error(`Failed to delete Exchange: ${error.body?.message || error.message}`);
    }
  }

  /**
   * Create an Exchange and return a connected client.
   */
  async createExchangeClient(req: ExchangeRequest): Promise<ExchangeClient> {
    await this.createExchange(req);
    const resource = await this.waitForExchangeReady(
      req.name,
      req.namespace || 'default'
    );

    const nc = await connect({
      servers: resource.connectionInfo.natsURL,
      maxReconnectAttempts: -1,
      reconnectTimeWait: 2000,
    });
    const js = nc.jetstream();

    return new ExchangeClient(
      nc,
      js,
      resource.connectionInfo.inputSubject,
      resource.connectionInfo.outputSubject,
      resource.name,
      resource.namespace,
    );
  }

  async deleteExchangeClient(exchange: ExchangeClient): Promise<void> {
    await this.deleteExchange(exchange.name, exchange.namespace);
  }
}
