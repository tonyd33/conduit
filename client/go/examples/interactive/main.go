package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	conduit "github.com/tonyd33/conduit/client/go"
)

func main() {
	// Configuration
	workerImage := getEnv("WORKER_IMAGE", "circus-chimp")
	exchangeName := getEnv("EXCHANGE_NAME", "interactive-exchange")
	namespace := getEnv("NAMESPACE", "default")
	natsURL := getEnv("NATS_URL", "nats://conduit-nats.conduit-system.svc.cluster.local:4222")

	fmt.Println("Conduit Interactive Client")
	fmt.Println("===========================")
	fmt.Printf("Namespace: %s\n", namespace)
	fmt.Printf("NATS URL: %s\n", natsURL)
	fmt.Printf("Worker Image: %s\n", workerImage)
	fmt.Printf("Exchange Name: %s\n\n", exchangeName)

	// Create Conduit Kubernetes client
	conduitClient, err := conduit.NewConduit(natsURL, namespace)
	if err != nil {
		fmt.Printf("Error creating Conduit client: %v\n", err)
		os.Exit(1)
	}

	// Create Exchange via Kubernetes API
	fmt.Println("Creating Exchange...")
	client, err := conduitClient.CreateExchangeClient(conduit.ExchangeRequest{
		Name:      exchangeName,
		Namespace: namespace,
		Image:     workerImage,
	})
	if err != nil {
		fmt.Printf("Error creating Exchange: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Exchange created and ready!")
	fmt.Println("\nYou can now send messages to the Exchange.")
	fmt.Println("Type your message and press Enter. Press Ctrl+D or Ctrl+C to exit.")
	fmt.Println()

	// Set up cleanup on exit
	defer func() {
		fmt.Println("\n\nCleaning up...")
		fmt.Println("Closing connection...")
		client.Close()

		fmt.Println("Deleting Exchange...")
		if err := conduitClient.DeleteExchangeClient(client); err != nil {
			fmt.Printf("Error deleting Exchange: %v\n", err)
		} else {
			fmt.Println("Exchange deleted successfully")
		}
	}()

	// Handle interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal...")
		cancel()
	}()

	// Subscribe to responses in background
	responseChan := make(chan string, 10)
	go func() {
		client.Subscribe(ctx, func(msg *conduit.Message) error {
			if msg.Type == conduit.MessageTypeData {
				responseChan <- string(msg.Payload)
			}
			return nil
		})
	}()

	// Start response printer
	go func() {
		for response := range responseChan {
			fmt.Printf("\n<< %s\n", response)
			fmt.Print(">> ")
		}
	}()

	// Read user input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(">> ")

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			fmt.Print(">> ")
			continue
		}

		// Send message
		if err := client.Send(ctx, text); err != nil {
			fmt.Printf("Error sending message: %v\n", err)
		}

		fmt.Print(">> ")
	}

	// Check for EOF (Ctrl+D) or error
	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Printf("Error reading input: %v\n", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
