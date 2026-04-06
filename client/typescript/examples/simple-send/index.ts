/**
 * Simple example of creating an Exchange and sending messages using the Conduit TypeScript client.
 *
 * This demonstrates:
 * 1. Creating an Exchange via Kubernetes API
 * 2. Sending messages to the Exchange
 * 3. Subscribing to responses
 * 4. Cleaning up resources
 */

import { Conduit } from '@mnke/conduit-client';

async function main() {
  // Configuration from environment variables
  const workerImage = process.env.WORKER_IMAGE || 'conduit/examples/echo:go';
  const exchangeName = process.env.EXCHANGE_NAME || 'simple-send-ts';
  const namespace = process.env.NAMESPACE || 'default';
  const natsURL = process.env.NATS_URL || 'nats://conduit-nats.conduit-system.svc.cluster.local:4222';

  console.log('Conduit Simple Send Example (TypeScript)');
  console.log('========================================');
  console.log(`Namespace: ${namespace}`);
  console.log(`NATS URL: ${natsURL}`);
  console.log(`Worker Image: ${workerImage}`);
  console.log(`Exchange Name: ${exchangeName}\n`);

  const conduit = new Conduit({ namespace, natsURL });

  // Create Exchange via Kubernetes API
  console.log('Creating Exchange...');
  const exchange = await conduit.createExchangeClient({
    name: exchangeName,
    namespace: namespace,
    image: workerImage,
  });
  console.log('Exchange created and ready!\n');

  // Track if response received
  let responseReceived = false;
  const responsePromise = new Promise<void>((resolve) => {
    // Subscribe to responses in background
    exchange
      .subscribe(async (msg) => {
        console.log(`Received response: ${JSON.stringify(msg.payload)}`);
        responseReceived = true;
        resolve();
      })
      .catch((err) => {
        console.error('Subscription error:', err);
      });
  });

  try {
    // Send a message
    console.log('Sending message...');
    await exchange.send({ message: 'Hello from TypeScript!' });
    console.log('Message sent!\n');

    // Wait for response with timeout
    const timeout = new Promise<void>((_, reject) =>
      setTimeout(() => reject(new Error('timeout')), 10000)
    );

    try {
      await Promise.race([responsePromise, timeout]);
    } catch (err: any) {
      if (err.message === 'timeout') {
        console.log('No response received within timeout');
      } else {
        throw err;
      }
    }
  } finally {
    // Clean up
    console.log('\nCleaning up...');

    // Close connection
    console.log('Closing connection...');
    await exchange.close();

    // Delete Exchange
    console.log('Deleting Exchange...');
    await conduit.deleteExchangeClient(exchange);
    console.log('Done!');
  }
}

main().catch((err) => {
  console.error('Error:', err);
  process.exit(1);
});
