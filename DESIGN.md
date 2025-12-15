# Realtime SDK - Design Document

## Overview

This document explains the architectural decisions, design patterns, and implementation details of the Realtime Messaging SDK for Go.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Design Patterns](#design-patterns)
3. [Concurrency Model](#concurrency-model)
4. [Error Handling](#error-handling)
5. [Testing Strategy](#testing-strategy)
6. [API Design Principles](#api-design-principles)

---

## Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                              │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    client.Client                         │    │
│  │  - SendMessage()    - Connect()    - Close()            │    │
│  │  - On()             - JoinChannel() - StartStream()     │    │
│  └─────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│                     Middleware Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Logging    │  │   Recovery   │  │   Timeout    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│                      Event Layer                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                 observer.EventBus                        │    │
│  │  - Subscribe()  - Notify()  - Unsubscribe()             │    │
│  └─────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│                    Connection Layer                              │
│  ┌──────────────────────┐  ┌────────────────────────────┐      │
│  │  connection.Manager  │  │  strategy.ConnectionStrategy│      │
│  │  - Connect()         │  │  - WebSocketStrategy        │      │
│  │  - Reconnect()       │  │  - HTTPPollingStrategy      │      │
│  │  - Send()            │  └────────────────────────────┘      │
│  └──────────────────────┘                                       │
├─────────────────────────────────────────────────────────────────┤
│                      Data Layer                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Message    │  │    Event     │  │   Protocol   │          │
│  │   Factory    │  │   Factory    │  │    Frame     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

### Package Structure

| Package | Responsibility |
|---------|---------------|
| `pkg/client` | Main SDK entry point, high-level API |
| `pkg/config` | Configuration management with validation |
| `pkg/connection` | Connection lifecycle and reconnection logic |
| `pkg/events` | Event types and structures |
| `pkg/handlers` | Message handling and middleware chain |
| `pkg/message` | Message types, serialization, builder |
| `pkg/mock` | Mock server for testing |
| `pkg/patterns/*` | Design pattern implementations |
| `internal/protocol` | Wire protocol definitions |
| `internal/utils` | Utility functions (Snowflake, rate limiter) |

---

## Design Patterns

### 1. Builder Pattern

**Location:** `pkg/config/config.go`, `pkg/message/message.go`, `pkg/client/client.go`

**Purpose:** Provides a fluent, readable way to construct complex objects with many optional parameters.

**Implementation:**

```go
// Config Builder
sdk, err := client.NewClientBuilder().
    WithAPIKey("my-api-key").
    WithEndpoint("wss://example.com/realtime").
    WithReconnectDelay(time.Second * 5).
    WithMaxReconnects(10).
    Build()

// Message Builder
msg := message.NewBuilder().
    WithChannel("general").
    WithContent("Hello!").
    WithPriority(message.HighPriority).
    WithMetadata("key", "value").
    Build()
```

**Benefits:**
- Self-documenting code
- Compile-time type safety
- Default values easily applied
- Immutable configuration after build

---

### 2. Factory Pattern

**Location:** `pkg/patterns/factory/factory.go`

**Purpose:** Centralizes object creation and encapsulates construction logic.

**Implementation:**

```go
// Message Factory
factory := factory.NewMessageFactory("user-123")
textMsg := factory.CreateTextMessage("general", "Hello!")
imageMsg := factory.CreateImageMessage("photos", "https://example.com/img.jpg")
systemMsg := factory.CreateSystemMessage("general", "User joined")

// Abstract Factory
realtimeFactory := factory.NewRealtimeFactory("user-123")
msg, _ := realtimeFactory.CreateMessage(message.TypeText, "general", "Hello!")
event, _ := realtimeFactory.CreateEvent(events.MessageReceived, payload)
```

**Benefits:**
- Consistent object creation
- Encapsulates complex initialization
- Easy to add new message types
- Testable through dependency injection

---

### 3. Observer Pattern

**Location:** `pkg/patterns/observer/observer.go`

**Purpose:** Decouples event producers from consumers, enabling flexible event handling.

**Implementation:**

```go
// EventBus - Subject
eventBus := observer.NewEventBus()

// Subscribe - Observer registration
unsubscribe := eventBus.Subscribe(events.MessageReceived, func(e events.Event) {
    fmt.Printf("Received: %v\n", e.Payload)
})

// Notify - Event dispatch
eventBus.Notify(events.NewEvent(events.MessageReceived, message))

// Unsubscribe
unsubscribe()
```

**Key Features:**
- Thread-safe subscription management
- Async event dispatch (goroutines)
- Wildcard subscriptions (all events)
- Automatic cleanup with unsubscribe function

---

### 4. Strategy Pattern

**Location:** `pkg/patterns/strategy/strategy.go`

**Purpose:** Allows runtime selection of different connection algorithms.

**Implementation:**

```go
// Strategy Interface
type ConnectionStrategy interface {
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Send(ctx context.Context, msg *message.Message) error
    Receive() <-chan *message.Message
    IsConnected() bool
}

// Concrete Strategies
wsStrategy := strategy.NewWebSocketStrategy(cfg)     // Real-time
httpStrategy := strategy.NewHTTPPollingStrategy(cfg, pollInterval) // Fallback

// Strategy Selector
selector := strategy.NewStrategySelector()
selector.Register("websocket", wsStrategy)
selector.Register("http", httpStrategy)
selected, _ := selector.Select("websocket")
```

**Benefits:**
- Easy to add new connection types (SSE, gRPC, etc.)
- Runtime strategy switching
- Testable with mock strategies
- Open/Closed principle compliance

---

### 5. Middleware Pattern

**Location:** `pkg/handlers/handlers.go`

**Purpose:** Allows composable, reusable request/response processing.

**Implementation:**

```go
// Middleware type
type Middleware func(MessageHandler) MessageHandler

// Built-in middlewares
sdk.Use(handlers.LoggingMiddleware(logger))
sdk.Use(handlers.RecoveryMiddleware(panicHandler))
sdk.Use(handlers.TimeoutMiddleware(ctx))
sdk.Use(handlers.FilterMiddleware(func(msg *message.Message) bool {
    return msg.Type == message.TypeText
}))
```

**Chain Execution:**
```
Request → Logging → Recovery → Timeout → Handler → Response
```

---

## Concurrency Model

### Thread Safety

All public methods are thread-safe through careful use of:

1. **sync.RWMutex** - For read-heavy data (connection state, channels)
2. **sync.Mutex** - For write-heavy operations (sequences, counters)
3. **Channels** - For message passing between goroutines

```go
// Example: Connection Manager
type Manager struct {
    mu    sync.RWMutex
    state State
    // ...
}

func (m *Manager) IsConnected() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.state == StateConnected
}
```

### Goroutine Management

```
┌─────────────────────────────────────────────────────┐
│                    Main Client                       │
├─────────────────────────────────────────────────────┤
│  goroutine: Message Reader                          │
│  - Reads from WebSocket                             │
│  - Forwards to message channel                      │
├─────────────────────────────────────────────────────┤
│  goroutine: Message Writer                          │
│  - Reads from send queue                            │
│  - Writes to WebSocket                              │
├─────────────────────────────────────────────────────┤
│  goroutine: Event Dispatcher                        │
│  - Dispatches events to subscribers                 │
│  - Non-blocking async notifications                 │
├─────────────────────────────────────────────────────┤
│  goroutine: Heartbeat                               │
│  - Sends periodic pings                             │
│  - Monitors connection health                       │
└─────────────────────────────────────────────────────┘
```

### Graceful Shutdown

```go
func (c *Client) Close() error {
    c.mu.Lock()
    if c.closed {
        c.mu.Unlock()
        return nil
    }
    c.closed = true
    close(c.closeChan)  // Signal all goroutines
    c.mu.Unlock()

    // Wait for goroutines with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return c.connManager.Close(ctx)
}
```

---

## Error Handling

### Error Types

```go
// Sentinel errors for type checking
var (
    ErrNotConnected     = errors.New("client: not connected")
    ErrAlreadyConnected = errors.New("client: already connected")
    ErrChannelNotFound  = errors.New("client: channel not found")
    ErrRateLimited      = errors.New("client: rate limited")
    ErrClosed           = errors.New("client: client is closed")
)

// Usage with errors.Is
if errors.Is(err, client.ErrNotConnected) {
    // Handle not connected
}
```

### Error Propagation

1. **Internal errors** - Wrapped with context
2. **Network errors** - Trigger reconnection
3. **Protocol errors** - Surfaced to user
4. **Validation errors** - Returned immediately

### Retry Logic

```go
// Exponential backoff for reconnection
func (m *Manager) calculateBackoff(attempt int) time.Duration {
    backoff := float64(m.config.ReconnectDelay) * math.Pow(2, float64(attempt-1))
    maxBackoff := float64(60 * time.Second)
    if backoff > maxBackoff {
        backoff = maxBackoff
    }
    return time.Duration(backoff)
}
```

---

## Testing Strategy

### Test Pyramid

```
          /\
         /  \
        / E2E \        <- Integration tests with mock server
       /──────\
      /  Unit  \       <- Unit tests for each component
     /──────────\
    / Component  \     <- Tests for patterns, utils
   /──────────────\
```

### Unit Tests

- Located in `test/unit/`
- Test individual components in isolation
- Use mocks for dependencies
- High coverage target (>80%)

```go
func TestMessageBuilder(t *testing.T) {
    msg := message.NewBuilder().
        WithChannel("test").
        WithContent("Hello").
        Build()
    
    assert.Equal(t, "test", msg.Channel)
    assert.Equal(t, "Hello", msg.Content)
}
```

### Integration Tests

- Located in `test/integration/`
- Use mock WebSocket server
- Test full client lifecycle
- Verify event flow

```go
func TestClientWithMockServer(t *testing.T) {
    server := mock.NewServer("localhost:18080")
    server.Start()
    defer server.Stop()
    
    sdk := client.NewClient(config.Config{
        APIKey:   "test-key",
        Endpoint: server.URL(),
    })
    
    err := sdk.Connect()
    require.NoError(t, err)
    defer sdk.Close()
    
    assert.True(t, sdk.IsConnected())
}
```

### Mock Server

The `pkg/mock` package provides a full WebSocket server for testing:

- Handles connections and authentication
- Routes messages between clients
- Simulates events
- Records messages for assertions

---

## API Design Principles

### 1. Simple Common Case

```go
// Most common usage is simplest
sdk := client.NewClient(config.Config{
    APIKey:   "key",
    Endpoint: "wss://example.com/ws",
})
sdk.Connect()
sdk.SendMessage("general", "Hello!")
sdk.Close()
```

### 2. Progressive Disclosure

```go
// Basic: Simple message
sdk.SendMessage("general", "Hello!")

// Advanced: With options
sdk.SendMessageWithOptions("general", "Hello!", message.Options{
    Priority: message.HighPriority,
    TTL:      time.Minute * 5,
})

// Expert: With context
ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
defer cancel()
sdk.SendMessageWithContext(ctx, "general", "Hello!")
```

### 3. Context Support

All blocking operations accept `context.Context`:

```go
func (c *Client) ConnectWithContext(ctx context.Context) error
func (c *Client) SendMessageWithContext(ctx context.Context, channel, content string) error
func (c *Client) CloseWithContext(ctx context.Context) error
```

### 4. Resource Cleanup

- `Close()` method for graceful shutdown
- `defer` friendly design
- Unsubscribe functions for event handlers
- No resource leaks

### 5. Idiomatic Go

- Error returns instead of exceptions
- Interface-based design
- Small interfaces (1-3 methods)
- Exported types are minimal
- Internal implementation details hidden

---

## Future Improvements

1. **Connection Pooling** - Multiple connections for higher throughput
2. **Compression** - WebSocket message compression
3. **Metrics** - Prometheus metrics export
4. **Tracing** - OpenTelemetry integration
5. **Persistent Queue** - Offline message queuing
6. **E2E Encryption** - End-to-end message encryption
