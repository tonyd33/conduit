/**
 * Conduit client for managing Exchanges.
 */

import { ExchangeRequest, ExchangeResource } from './api';
import { ExchangeClient } from './exchange';
import { connect } from 'nats';

export class Conduit {
  private apiBaseUrl: string;

  constructor({ apiBaseUrl }: { apiBaseUrl: string }) {
    this.apiBaseUrl = apiBaseUrl;
  }

  /**
   * Create Exchange from the API.
   * @private
   */
  private async createExchange(req: ExchangeRequest): Promise<ExchangeResource> {
    const url = `${this.apiBaseUrl}/api/v1/exchanges`;

    const headers = new Headers({
      'Content-Type': 'application/json',
    });

    const body = {
      name: req.name,
      namespace: req.namespace || 'default',
      image: req.image,
      ...(req.env ? { env: req.env } : {}),
    };

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
    });

    if (response.status !== 201) {
      const text = await response.text();
      throw new Error(
        `Failed to create Exchange (status ${response.status}): ${text}`
      );
    }

    return (await response.json()) as ExchangeResource;
  }

  /**
   * Get Exchange information from the API.
   * @private
   */
  private async getExchange(
    name: string,
    namespace: string
  ): Promise<ExchangeResource> {
    const url = `${this.apiBaseUrl}/api/v1/exchanges/${namespace}/${name}`;

    const headers = new Headers({
      'Content-Type': 'application/json',
    });

    const response = await fetch(url, { headers });

    if (response.status !== 200) {
      const text = await response.text();
      throw new Error(`Failed to get Exchange (status ${response.status}): ${text}`);
    }

    return (await response.json()) as ExchangeResource;
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
   * Delete an Exchange via the API.
   */
  private async deleteExchange(name: string, namespace: string): Promise<void> {
    const url = `${this.apiBaseUrl}/api/v1/exchanges/${namespace}/${name}`;

    const headers = new Headers({
      'Content-Type': 'application/json',
    });

    const response = await fetch(url, {
      method: 'DELETE',
      headers,
    });

    if (response.status !== 204) {
      const text = await response.text();
      throw new Error(`API error (status ${response.status}): ${text}`);
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
