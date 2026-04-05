package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	conduit "github.com/tonyd33/conduit/sdk/go"
)

func main() {
	log.Println("Starting Echo Exchange application...")

	// Create Conduit client
	client, err := conduit.NewClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Define message handler
	handler := func(ctx context.Context, msg *conduit.Message) error {
		log.Printf("Received message (seq=%d, type=%s)", msg.Sequence, msg.Type)

		// Parse payload as generic JSON (accept any structure)
		var payload interface{}
		if err := msg.UnmarshalPayload(&payload); err != nil {
			log.Printf("Failed to unmarshal payload: %v", err)
			return err
		}

		log.Printf("Payload: %+v", payload)

		// Echo the message back
		response := map[string]interface{}{
			"echo":     payload,
			"sequence": msg.Sequence,
		}

		if err := client.Publish(ctx, response); err != nil {
			log.Printf("Failed to publish response: %v", err)
			return err
		}

		return nil
	}

	// Start processing
	log.Println("Ready to process messages")
	if err := client.Run(ctx, handler); err != nil && err != context.Canceled {
		log.Fatalf("Client error: %v", err)
	}

	log.Println("Echo application shut down successfully")
}
