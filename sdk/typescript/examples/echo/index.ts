/**
 * Echo Exchange Example
 *
 * A simple example application demonstrating the Conduit TypeScript SDK.
 */

import { Client, Message } from '@mnke/conduit-sdk';

async function main() {
  console.log('Starting Echo Exchange application...');

  // Create the Conduit client
  const client = await Client.create();

  // Handle shutdown signals
  let isShuttingDown = false;

  const shutdown = async () => {
    if (isShuttingDown) return;
    isShuttingDown = true;
    console.log('Shutting down...');
    await client.close();
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  // Define message handler
  const handler = async (msg: Message) => {
    console.log(`Received message (seq=${msg.sequence}, type=${msg.type})`);

    // Log the payload
    console.log('Payload:', msg.payload);

    // Echo the message back
    const response = {
      echo: msg.payload,
      sequence: msg.sequence,
    };

    await client.publish(response);
  };

  try {
    // Start processing messages
    console.log('Ready to process messages');
    await client.run(handler);
  } catch (error) {
    console.error('Error:', error);
    await client.close();
    process.exit(1);
  }
}

// Run the application
main().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
