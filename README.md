# Realtime Messaging SDK for Go

A production-ready Golang SDK for real-time messaging and video streaming, featuring clean architecture, design patterns, and comprehensive test coverage.

## Features

- 🚀 **Easy Initialization** - Simple builder pattern for SDK configuration
- 📨 **Real-time Messaging** - Send and receive messages with WebSocket
- 🎥 **Video Streaming** - Stream video events in real-time
- 🔄 **Auto-Reconnection** - Automatic reconnection with exponential backoff
- 📊 **Event System** - Observer pattern for flexible event handling
- 🧪 **Mock Backend** - Built-in mock server for testing
- ✅ **Comprehensive Tests** - Unit and integration tests included

## Installation

```bash
go get github.com/rinaldypasya/realtime-sdk
```

## Configuration

The SDK requires API credentials to connect to the realtime server. You can configure these using environment variables:

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and add your credentials
REALTIME_API_KEY=your-api-key-here
REALTIME_ENDPOINT=wss://your-server.com/realtime
```

**Note:** Never commit your `.env` file to version control. It's already included in `.gitignore`.

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/rinaldypasya/realtime-sdk/pkg/client"
    "github.com/rinaldypasya/realtime-sdk/pkg/config"
    "github.com/rinaldypasya/realtime-sdk/pkg/events"
)

func main() {
    // Load credentials from environment variables
    apiKey := os.Getenv("REALTIME_API_KEY")
    endpoint := os.Getenv("REALTIME_ENDPOINT")

    // Initialize the SDK with configuration
    sdk := client.NewClient(config.Config{
        APIKey:   apiKey,
        Endpoint: endpoint,
    })

    // Register event handlers
    sdk.On(events.MessageReceived, func(e events.Event) {
        fmt.Printf("Received message: %s\n", e.Payload)
    })

    // Connect to the server
    if err := sdk.Connect(); err != nil {
        log.Fatal(err)
    }
    defer sdk.Close()

    // Send a message
    err := sdk.SendMessage("general", "Hello, world!")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Running the Examples

```bash
# Set environment variables
export REALTIME_API_KEY=your-api-key
export REALTIME_ENDPOINT=wss://your-server.com/realtime

# Run the example application
go run cmd/example/main.go
```

## Architecture

### Design Patterns Used

1. **Builder Pattern** - For flexible SDK configuration
2. **Factory Pattern** - For creating message and event objects
3. **Observer Pattern** - For event handling and subscriptions
4. **Strategy Pattern** - For different connection strategies (WebSocket, HTTP polling)
5. **Singleton Pattern** - For connection manager

### Project Structure

```
realtime-sdk/
├── cmd/
│   └── example/           # Example applications
├── pkg/
│   ├── client/            # Main SDK client
│   ├── config/            # Configuration management
│   ├── connection/        # Connection management
│   ├── events/            # Event system
│   ├── handlers/          # Message handlers
│   ├── message/           # Message types and factories
│   ├── mock/              # Mock backend server
│   └── patterns/          # Design pattern implementations
│       ├── factory/
│       ├── observer/
│       └── strategy/
├── internal/
│   ├── protocol/          # Protocol implementations
│   └── utils/             # Utility functions
└── test/
    ├── integration/       # Integration tests
    └── unit/              # Unit tests
```

## API Reference

### Client Initialization

```go
// Using Config struct
sdk := client.NewClient(config.Config{
    APIKey:          "my-api-key",
    Endpoint:        "wss://example.com/realtime",
    ReconnectDelay:  time.Second * 5,
    MaxReconnects:   10,
    HeartbeatInterval: time.Second * 30,
})

// Using Builder pattern
sdk := client.NewClientBuilder().
    WithAPIKey("my-api-key").
    WithEndpoint("wss://example.com/realtime").
    WithReconnectDelay(time.Second * 5).
    WithMaxReconnects(10).
    Build()
```

### Sending Messages

```go
// Send a simple text message
err := sdk.SendMessage("channel-name", "Hello, world!")

// Send a message with options
err := sdk.SendMessageWithOptions("channel-name", "Hello!", message.Options{
    Priority: message.HighPriority,
    TTL:      time.Minute * 5,
})
```

### Video Streaming

```go
// Start a video stream
stream, err := sdk.StartStream("stream-id")
if err != nil {
    log.Fatal(err)
}

// Send video frame
err = stream.SendFrame(frameData)

// End stream
stream.End()
```

### Event Handling

```go
// Subscribe to events
sdk.On(events.MessageReceived, func(e events.Event) {
    msg := e.Payload.(*message.Message)
    fmt.Printf("Channel: %s, Content: %s\n", msg.Channel, msg.Content)
})

sdk.On(events.ConnectionLost, func(e events.Event) {
    fmt.Println("Connection lost, attempting to reconnect...")
})

// Unsubscribe
unsubscribe := sdk.On(events.MessageReceived, handler)
unsubscribe() // Remove the handler
```

### Graceful Shutdown

```go
// Close the connection gracefully
sdk.Close()

// Close with timeout
ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
defer cancel()
sdk.CloseWithContext(ctx)
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `APIKey` | string | required | API key for authentication |
| `Endpoint` | string | required | WebSocket endpoint URL |
| `ReconnectDelay` | Duration | 5s | Initial delay before reconnecting |
| `MaxReconnects` | int | 10 | Maximum reconnection attempts |
| `HeartbeatInterval` | Duration | 30s | Interval between heartbeat pings |
| `ReadBufferSize` | int | 1024 | WebSocket read buffer size |
| `WriteBufferSize` | int | 1024 | WebSocket write buffer size |

## Testing

### Run Unit Tests

```bash
go test ./test/unit/... -v
```

### Run Integration Tests

```bash
go test ./test/integration/... -v
```

### Run All Tests with Coverage

```bash
go test ./... -v -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Error Handling

```go
err := sdk.SendMessage("channel", "message")
if err != nil {
    switch {
    case errors.Is(err, client.ErrNotConnected):
        // Handle not connected error
    case errors.Is(err, client.ErrChannelNotFound):
        // Handle channel not found error
    case errors.Is(err, client.ErrRateLimited):
        // Handle rate limiting
    default:
        // Handle other errors
    }
}
```

## Concurrency Safety

The SDK is designed to be safe for concurrent use:

- All public methods are thread-safe
- Internal state is protected by mutexes
- Channel operations are properly synchronized
- Graceful shutdown waits for pending operations

## License

MIT License - Copyright (c) 2025 Rinaldy Pasya

See [LICENSE](LICENSE) file for details.
