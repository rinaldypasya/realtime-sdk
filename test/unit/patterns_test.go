package unit

import (
	"sync"
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/factory"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/observer"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/strategy"
	"github.com/stretchr/testify/assert"
)

// Observer Pattern Tests

func TestEventBusSubscribe(t *testing.T) {
	bus := observer.NewEventBus()

	var received events.Event
	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(events.MessageReceived, func(e events.Event) {
		received = e
		wg.Done()
	})

	bus.Notify(events.NewEvent(events.MessageReceived, "test payload"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, events.MessageReceived, received.Type)
		assert.Equal(t, "test payload", received.Payload)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := observer.NewEventBus()

	callCount := 0
	unsubscribe := bus.Subscribe(events.MessageReceived, func(e events.Event) {
		callCount++
	})

	// Unsubscribe
	unsubscribe()

	// Notify synchronously
	bus.NotifySync(events.NewEvent(events.MessageReceived, nil))

	assert.Equal(t, 0, callCount)
}

func TestEventBusSubscribeAll(t *testing.T) {
	bus := observer.NewEventBus()

	var receivedEvents []events.Event
	var mu sync.Mutex

	bus.SubscribeAll(func(e events.Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	bus.NotifySync(events.NewEvent(events.MessageReceived, nil))
	bus.NotifySync(events.NewEvent(events.Connected, nil))
	bus.NotifySync(events.NewEvent(events.UserTyping, nil))

	assert.Len(t, receivedEvents, 3)
}

func TestEventBusHasSubscribers(t *testing.T) {
	bus := observer.NewEventBus()

	assert.False(t, bus.HasSubscribers(events.MessageReceived))

	unsubscribe := bus.Subscribe(events.MessageReceived, func(e events.Event) {})

	assert.True(t, bus.HasSubscribers(events.MessageReceived))
	assert.False(t, bus.HasSubscribers(events.Connected))

	unsubscribe()
	assert.False(t, bus.HasSubscribers(events.MessageReceived))
}

func TestEventBusClear(t *testing.T) {
	bus := observer.NewEventBus()

	bus.Subscribe(events.MessageReceived, func(e events.Event) {})
	bus.Subscribe(events.Connected, func(e events.Event) {})
	bus.SubscribeAll(func(e events.Event) {})

	bus.Clear()

	assert.False(t, bus.HasSubscribers(events.MessageReceived))
	assert.False(t, bus.HasSubscribers(events.Connected))
}

// Factory Pattern Tests

func TestMessageFactory(t *testing.T) {
	f := factory.NewMessageFactory("user-123")

	t.Run("CreateTextMessage", func(t *testing.T) {
		msg := f.CreateTextMessage("general", "Hello!")
		assert.NotEmpty(t, msg.ID)
		assert.Equal(t, message.TypeText, msg.Type)
		assert.Equal(t, "general", msg.Channel)
		assert.Equal(t, "Hello!", msg.Content)
		assert.Equal(t, "user-123", msg.SenderID)
		assert.Equal(t, message.StatusPending, msg.Status)
	})

	t.Run("CreateImageMessage", func(t *testing.T) {
		msg := f.CreateImageMessage("photos", "https://example.com/image.jpg")
		assert.Equal(t, message.TypeImage, msg.Type)
		assert.Equal(t, "image", msg.Metadata["type"])
	})

	t.Run("CreateVideoMessage", func(t *testing.T) {
		msg := f.CreateVideoMessage("videos", "https://example.com/video.mp4")
		assert.Equal(t, message.TypeVideo, msg.Type)
		assert.Equal(t, "video", msg.Metadata["type"])
	})

	t.Run("CreateFileMessage", func(t *testing.T) {
		msg := f.CreateFileMessage("files", "document.pdf", "https://example.com/doc.pdf", 1024)
		assert.Equal(t, message.TypeFile, msg.Type)
		assert.Equal(t, "document.pdf", msg.Metadata["name"])
		assert.Equal(t, int64(1024), msg.Metadata["size"])
	})

	t.Run("CreateSystemMessage", func(t *testing.T) {
		msg := f.CreateSystemMessage("general", "User joined")
		assert.Equal(t, message.TypeSystem, msg.Type)
		assert.Equal(t, "system", msg.SenderID)
		assert.Equal(t, message.StatusDelivered, msg.Status)
	})

	t.Run("CreateReplyMessage", func(t *testing.T) {
		msg := f.CreateReplyMessage("general", "I agree!", "original-msg-id")
		assert.Equal(t, message.TypeText, msg.Type)
		assert.Equal(t, "original-msg-id", msg.ReplyTo)
	})
}

func TestEventFactory(t *testing.T) {
	f := factory.NewEventFactory()

	t.Run("CreateConnectionEvent", func(t *testing.T) {
		event := f.CreateConnectionEvent(events.Connected, "wss://example.com", 0)
		assert.Equal(t, events.Connected, event.Type)
		connEvent := event.Payload.(*events.ConnectionEvent)
		assert.Equal(t, "wss://example.com", connEvent.Endpoint)
	})

	t.Run("CreatePresenceEvent", func(t *testing.T) {
		event := f.CreatePresenceEvent(events.UserOnline, "user-123", "john", "general", "online")
		assert.Equal(t, events.UserOnline, event.Type)
		presenceEvent := event.Payload.(*events.PresenceEvent)
		assert.Equal(t, "user-123", presenceEvent.UserID)
		assert.Equal(t, "john", presenceEvent.Username)
	})
}

func TestRealtimeFactory(t *testing.T) {
	f := factory.NewRealtimeFactory("user-123")

	t.Run("CreateMessage", func(t *testing.T) {
		msg, err := f.CreateMessage(message.TypeText, "general", "Hello!")
		assert.NoError(t, err)
		assert.Equal(t, message.TypeText, msg.Type)
	})

	t.Run("CreateMessageUnknownType", func(t *testing.T) {
		_, err := f.CreateMessage("unknown", "general", "Hello!")
		assert.Error(t, err)
	})

	t.Run("CreateEvent", func(t *testing.T) {
		event, err := f.CreateEvent(events.MessageReceived, "payload")
		assert.NoError(t, err)
		assert.Equal(t, events.MessageReceived, event.Type)
	})
}

func TestStreamFactory(t *testing.T) {
	f := factory.NewStreamFactory(1000000) // 1 Mbps default bitrate

	t.Run("CreateVideoStream", func(t *testing.T) {
		stream := f.CreateVideoStream("video")
		assert.NotEmpty(t, stream.ID)
		assert.Equal(t, "video", stream.Type)
		assert.Equal(t, 1000000, stream.Bitrate)
		assert.True(t, stream.Active)
	})

	t.Run("CreateFrame", func(t *testing.T) {
		frame := f.CreateFrame("stream-123", 42, []byte("frame-data"))
		assert.NotEmpty(t, frame.ID)
		assert.Equal(t, "stream-123", frame.StreamID)
		assert.Equal(t, int64(42), frame.Sequence)
		assert.Equal(t, []byte("frame-data"), frame.Data)
	})
}

// Strategy Pattern Tests

func TestWebSocketStrategy(t *testing.T) {
	cfg := config.Config{
		APIKey:   "test-key",
		Endpoint: "wss://example.com/ws",
	}

	ws := strategy.NewWebSocketStrategy(cfg)

	assert.Equal(t, "websocket", ws.Type())
	assert.False(t, ws.IsConnected())
}

func TestHTTPPollingStrategy(t *testing.T) {
	cfg := config.Config{
		APIKey:   "test-key",
		Endpoint: "https://example.com/api",
	}

	hp := strategy.NewHTTPPollingStrategy(cfg, time.Second)

	assert.Equal(t, "http_polling", hp.Type())
	assert.False(t, hp.IsConnected())
}

func TestStrategySelector(t *testing.T) {
	cfg := config.Config{
		APIKey:   "test-key",
		Endpoint: "wss://example.com/ws",
	}

	selector := strategy.NewStrategySelector()

	ws := strategy.NewWebSocketStrategy(cfg)
	hp := strategy.NewHTTPPollingStrategy(cfg, time.Second)

	selector.Register("websocket", ws)
	selector.Register("http", hp)

	t.Run("SelectWebSocket", func(t *testing.T) {
		selected, err := selector.Select("websocket")
		assert.NoError(t, err)
		assert.Equal(t, "websocket", selected.Type())
	})

	t.Run("SelectHTTP", func(t *testing.T) {
		selected, err := selector.Select("http")
		assert.NoError(t, err)
		assert.Equal(t, "http_polling", selected.Type())
	})

	t.Run("SelectUnknown", func(t *testing.T) {
		_, err := selector.Select("unknown")
		assert.Error(t, err)
	})

	t.Run("Current", func(t *testing.T) {
		selector.Select("websocket")
		current := selector.Current()
		assert.Equal(t, "websocket", current.Type())
	})
}
