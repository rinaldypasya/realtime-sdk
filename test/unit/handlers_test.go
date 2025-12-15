package unit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/handlers"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/stretchr/testify/assert"
)

func TestHandlerRegistry(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	t.Run("OnMessage", func(t *testing.T) {
		var called bool
		registry.OnMessage("test-channel", func(ctx context.Context, msg *message.Message) error {
			called = true
			return nil
		})

		msg := &message.Message{Channel: "test-channel"}
		err := registry.HandleMessage(context.Background(), msg)
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("OnAllMessages", func(t *testing.T) {
		registry := handlers.NewHandlerRegistry()

		var callCount int
		registry.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
			callCount++
			return nil
		})

		msg1 := &message.Message{Channel: "channel-1"}
		msg2 := &message.Message{Channel: "channel-2"}

		registry.HandleMessage(context.Background(), msg1)
		registry.HandleMessage(context.Background(), msg2)

		assert.Equal(t, 2, callCount)
	})

	t.Run("DefaultHandler", func(t *testing.T) {
		registry := handlers.NewHandlerRegistry()

		var defaultCalled bool
		registry.SetDefaultHandler(func(ctx context.Context, msg *message.Message) error {
			defaultCalled = true
			return nil
		})

		msg := &message.Message{Channel: "unhandled-channel"}
		err := registry.HandleMessage(context.Background(), msg)
		assert.NoError(t, err)
		assert.True(t, defaultCalled)
	})
}

func TestEventHandlerRegistry(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	var receivedEvent events.Event
	registry.OnEvent(events.MessageReceived, func(ctx context.Context, event events.Event) error {
		receivedEvent = event
		return nil
	})

	testEvent := events.NewEvent(events.MessageReceived, "test-payload")
	err := registry.HandleEvent(context.Background(), testEvent)

	assert.NoError(t, err)
	assert.Equal(t, events.MessageReceived, receivedEvent.Type)
}

func TestLoggingMiddleware(t *testing.T) {
	var logs []string
	var mu sync.Mutex

	logger := func(format string, args ...interface{}) {
		mu.Lock()
		logs = append(logs, format)
		mu.Unlock()
	}

	registry := handlers.NewHandlerRegistry()
	registry.Use(handlers.LoggingMiddleware(logger))
	registry.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
		return nil
	})

	msg := &message.Message{
		ID:      "msg-123",
		Channel: "general",
		Type:    message.TypeText,
	}

	registry.HandleMessage(context.Background(), msg)

	mu.Lock()
	assert.NotEmpty(t, logs)
	mu.Unlock()
}

func TestRecoveryMiddleware(t *testing.T) {
	var recovered interface{}

	registry := handlers.NewHandlerRegistry()
	registry.Use(handlers.RecoveryMiddleware(func(r interface{}) {
		recovered = r
	}))
	registry.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
		panic("test panic")
	})

	msg := &message.Message{Channel: "test"}
	err := registry.HandleMessage(context.Background(), msg)

	// Should not panic, error should be returned
	assert.NotNil(t, recovered)
	assert.Equal(t, "test panic", recovered)
	_ = err // Error handling depends on panic type
}

func TestFilterMiddleware(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	var processedCount int
	registry.Use(handlers.FilterMiddleware(func(msg *message.Message) bool {
		return msg.Type == message.TypeText
	}))
	registry.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
		processedCount++
		return nil
	})

	// Text message - should be processed
	textMsg := &message.Message{Type: message.TypeText}
	registry.HandleMessage(context.Background(), textMsg)
	assert.Equal(t, 1, processedCount)

	// Image message - should be filtered
	imageMsg := &message.Message{Type: message.TypeImage}
	registry.HandleMessage(context.Background(), imageMsg)
	assert.Equal(t, 1, processedCount) // Still 1
}

func TestTimeoutMiddleware(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	registry := handlers.NewHandlerRegistry()
	registry.Use(handlers.TimeoutMiddleware(ctx))
	registry.OnAllMessages(func(ctx context.Context, msg *message.Message) error {
		// Simulate slow processing
		time.Sleep(time.Millisecond * 100)
		return nil
	})

	msg := &message.Message{Channel: "test"}
	err := registry.HandleMessage(context.Background(), msg)

	// Should timeout
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestHandlerRegistryClear(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	var called bool
	registry.OnMessage("test", func(ctx context.Context, msg *message.Message) error {
		called = true
		return nil
	})

	registry.Clear()

	msg := &message.Message{Channel: "test"}
	registry.HandleMessage(context.Background(), msg)

	assert.False(t, called)
}

func TestMultipleHandlers(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	var order []int
	var mu sync.Mutex

	registry.OnMessage("test", func(ctx context.Context, msg *message.Message) error {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		return nil
	})

	registry.OnMessage("test", func(ctx context.Context, msg *message.Message) error {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		return nil
	})

	msg := &message.Message{Channel: "test"}
	registry.HandleMessage(context.Background(), msg)

	mu.Lock()
	assert.Equal(t, []int{1, 2}, order)
	mu.Unlock()
}

func TestHandlerError(t *testing.T) {
	registry := handlers.NewHandlerRegistry()

	expectedErr := errors.New("handler error")
	registry.OnMessage("test", func(ctx context.Context, msg *message.Message) error {
		return expectedErr
	})

	msg := &message.Message{Channel: "test"}
	err := registry.HandleMessage(context.Background(), msg)

	assert.Equal(t, expectedErr, err)
}
