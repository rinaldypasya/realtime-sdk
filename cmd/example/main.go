// Example application demonstrating the Realtime SDK usage.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/client"
	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/handlers"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
)

const (
	envAPIKey  = "REALTIME_API_KEY"
	envEndpoint = "REALTIME_ENDPOINT"

	msgSkipMissingCreds = "Skipping: REALTIME_API_KEY and REALTIME_ENDPOINT must be set"
	msgFailedToConnect  = "Failed to connect: %v"
	channelGeneral      = "general"
)

func main() {
	// Example 1: Basic Usage with Config struct
	basicUsageExample()

	// Example 2: Using Builder Pattern
	builderPatternExample()

	// Example 3: Event Handling
	eventHandlingExample()

	// Example 4: Video Streaming
	videoStreamingExample()

	// Example 5: Full Application
	fullApplicationExample()	
}

func basicUsageExample() {
	fmt.Println("\n=== Basic Usage Example ===")

	// Get credentials from environment variables
	apiKey := os.Getenv(envAPIKey)
	endpoint := os.Getenv(envEndpoint)

	if apiKey == "" || endpoint == "" {
		log.Println(msgSkipMissingCreds)
		return
	}

	// Initialize the SDK with configuration
	sdk := client.NewClient(config.Config{
		APIKey:   apiKey,
		Endpoint: endpoint,
	})

	// Connect to the server
	if err := sdk.Connect(); err != nil {
		log.Printf(msgFailedToConnect, err)
		return
	}
	defer sdk.Close()

	// Send a message
	err := sdk.SendMessage(channelGeneral, "Hello, world!")
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		return
	}

	fmt.Println("Message sent successfully!")
}

func builderPatternExample() {
	fmt.Println("\n=== Builder Pattern Example ===")

	apiKey := os.Getenv(envAPIKey)
	endpoint := os.Getenv(envEndpoint)

	if apiKey == "" || endpoint == "" {
		log.Println(msgSkipMissingCreds)
		return
	}

	// Use builder for more control
	sdk, err := client.NewClientBuilder().
		WithAPIKey(apiKey).
		WithEndpoint(endpoint).
		WithReconnectDelay(time.Second * 5).
		WithMaxReconnects(10).
		WithHeartbeatInterval(time.Second * 30).
		WithDebug(true).
		Build()

	if err != nil {
		log.Printf("Failed to build client: %v", err)
		return
	}
	defer sdk.Close()

	fmt.Printf("Client created with user ID: %s\n", sdk.UserID())
	fmt.Printf("Config: %+v\n", sdk.Config())
}

func eventHandlingExample() {
	fmt.Println("\n=== Event Handling Example ===")

	apiKey := os.Getenv(envAPIKey)
	endpoint := os.Getenv(envEndpoint)

	if apiKey == "" || endpoint == "" {
		log.Println(msgSkipMissingCreds)
		return
	}

	sdk := client.NewClient(config.Config{
		APIKey:   apiKey,
		Endpoint: endpoint,
	})

	// Register event handlers
	sdk.On(events.Connected, func(e events.Event) {
		fmt.Println("✓ Connected to server")
	})

	sdk.On(events.Disconnected, func(e events.Event) {
		fmt.Println("✗ Disconnected from server")
	})

	sdk.On(events.MessageReceived, func(e events.Event) {
		if msg, ok := e.Payload.(*message.Message); ok {
			fmt.Printf("📨 Message received: [%s] %s\n", msg.Channel, msg.Content)
		}
	})

	sdk.On(events.Reconnecting, func(e events.Event) {
		if connEvent, ok := e.Payload.(*events.ConnectionEvent); ok {
			fmt.Printf("🔄 Reconnecting... attempt %d\n", connEvent.ReconnectCount)
		}
	})

	sdk.On(events.ConnectionError, func(e events.Event) {
		fmt.Printf("❌ Connection error: %v\n", e.Error)
	})

	// Register unsubscribe example
	unsubscribe := sdk.On(events.UserTyping, func(e events.Event) {
		fmt.Println("Someone is typing...")
	})

	// Later: unsubscribe from typing events
	_ = unsubscribe // Call unsubscribe() to remove the handler

	if err := sdk.Connect(); err != nil {
		log.Printf(msgFailedToConnect, err)
		return
	}
	defer sdk.Close()

	fmt.Println("Event handlers registered!")
}

func videoStreamingExample() {
	fmt.Println("\n=== Video Streaming Example ===")

	apiKey := os.Getenv(envAPIKey)
	endpoint := os.Getenv(envEndpoint)

	if apiKey == "" || endpoint == "" {
		log.Println(msgSkipMissingCreds)
		return
	}

	sdk := client.NewClient(config.Config{
		APIKey:   apiKey,
		Endpoint: endpoint,
	})

	if err := sdk.Connect(); err != nil {
		log.Printf(msgFailedToConnect, err)
		return
	}
	defer sdk.Close()

	// Start a video stream
	stream, err := sdk.StartStream("my-stream-123")
	if err != nil {
		log.Printf("Failed to start stream: %v", err)
		return
	}

	fmt.Printf("Stream started: %s\n", stream.ID)

	// Simulate sending frames
	for i := 0; i < 5; i++ {
		frameData := []byte(fmt.Sprintf("frame-%d-data", i))
		if err := stream.SendFrame(frameData); err != nil {
			log.Printf("Failed to send frame: %v", err)
			break
		}
		fmt.Printf("Sent frame %d\n", i)
		time.Sleep(time.Millisecond * 100)
	}

	// End the stream
	stream.End()
	fmt.Println("Stream ended")
}

func fullApplicationExample() {
	fmt.Println("\n=== Full Application Example ===")

	apiKey := os.Getenv(envAPIKey)
	endpoint := os.Getenv(envEndpoint)

	if apiKey == "" || endpoint == "" {
		log.Println(msgSkipMissingCreds)
		return
	}

	// Create client with builder
	sdk, err := client.NewClientBuilder().
		WithAPIKey(apiKey).
		WithEndpoint(endpoint).
		WithReconnectDelay(time.Second * 3).
		WithMaxReconnects(5).
		Build()

	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Add middleware for logging
	sdk.Use(handlers.LoggingMiddleware(func(format string, args ...interface{}) {
		log.Printf("[SDK] "+format, args...)
	}))

	// Add recovery middleware
	sdk.Use(handlers.RecoveryMiddleware(func(r interface{}) {
		log.Printf("[SDK] Recovered from panic: %v", r)
	}))

	// Register message handlers for specific channels
	sdk.OnMessage("general", func(ctx context.Context, msg *message.Message) error {
		fmt.Printf("[general] %s: %s\n", msg.SenderID, msg.Content)
		return nil
	})

	sdk.OnMessage("announcements", func(ctx context.Context, msg *message.Message) error {
		fmt.Printf("[ANNOUNCEMENT] %s\n", msg.Content)
		return nil
	})

	// Register handler for all messages
	sdk.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
		// Log all messages
		log.Printf("Message: id=%s, channel=%s, type=%s", msg.ID, msg.Channel, msg.Type)
		return nil
	})

	// Event handlers
	sdk.On(events.Connected, func(e events.Event) {
		fmt.Println("Connected! Joining channels...")

		// Join channels after connecting
		sdk.JoinChannel("general")
		sdk.JoinChannel("announcements")
	})

	sdk.On(events.ChannelJoined, func(e events.Event) {
		if channel, ok := e.Payload.(string); ok {
			fmt.Printf("Joined channel: %s\n", channel)
		}
	})

	// Connect
	if err := sdk.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("Application started. Press Ctrl+C to exit.")

	// Send some messages
	go func() {
		time.Sleep(time.Second)

		sdk.SendMessage("general", "Hello everyone!")

		sdk.SendMessageWithOptions("general", "Important update!", message.Options{
			Priority: message.HighPriority,
			Metadata: map[string]interface{}{
				"category": "update",
			},
		})

		sdk.SendImage("general", "https://example.com/image.jpg")
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := sdk.CloseWithContext(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Application stopped.")
}
