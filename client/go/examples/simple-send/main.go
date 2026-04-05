package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	conduitclient "github.com/tonyd33/conduit/client/go"
)

func main() {
	// Configuration from environment variables
	apiURL := getEnv("API_URL", "http://localhost:8090")
	workerImage := getEnv("WORKER_IMAGE", "conduit/examples/echo:go")
	exchangeName := getEnv("EXCHANGE_NAME", "simple-send-go")

	fmt.Println("Conduit Simple Send Example (Go)")
	fmt.Println("=================================")
	fmt.Printf("API Server: %s\n", apiURL)
	fmt.Printf("Worker Image: %s\n", workerImage)
	fmt.Printf("Exchange Name: %s\n\n", exchangeName)

	conduit := conduitclient.NewConduit(apiURL, "")

	// Create Exchange via API
	fmt.Println("Creating Exchange...")
	exchange, err := conduit.CreateExchangeClient(conduitclient.ExchangeRequest{
		Name:      exchangeName,
		Namespace: "default",
		Image:     workerImage,
	})
	if err != nil {
		log.Fatalf("Failed to create Exchange: %v", err)
	}
	fmt.Println("Exchange created and ready!\n")

	// Track if response received
	responseReceived := make(chan bool, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to responses in background
	go func() {
		err := exchange.Subscribe(ctx, func(msg *conduitclient.Message) error {
			var payload map[string]interface{}
			if err := msg.UnmarshalPayload(&payload); err != nil {
				fmt.Printf("Error unmarshaling response: %v\n", err)
				return err
			}
			fmt.Printf("Received response: %+v\n", payload)
			responseReceived <- true
			return nil
		})
		if err != nil && err != context.Canceled {
			log.Printf("Subscribe error: %v", err)
		}
	}()

	// Send a message
	fmt.Println("Sending message...")
	if err := exchange.Send(context.Background(), map[string]interface{}{
		"message": "Hello from Go!",
	}); err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}
	fmt.Println("Message sent!\n")

	// Wait for response with timeout
	select {
	case <-responseReceived:
		// Response received
	case <-time.After(10 * time.Second):
		fmt.Println("No response received within timeout")
	}

	// Clean up
	fmt.Println("\nCleaning up...")

	// Close connection
	fmt.Println("Closing connection...")
	if err := exchange.Close(); err != nil {
		log.Printf("Failed to close connection: %v", err)
	}

	// Delete Exchange
	fmt.Println("Deleting Exchange...")
	if err := conduit.DeleteExchangeClient(exchange); err != nil {
		log.Printf("Failed to delete Exchange: %v", err)
	}
	fmt.Println("Done!")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
