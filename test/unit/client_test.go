package unit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/client"
	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/rinaldypasya/realtime-sdk/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAPIKey     = "test-api-key"
	testEndpoint   = "wss://test.example.com/realtime"
)

// setupMockServer starts a mock WebSocket server for testing
func setupMockServer(t *testing.T) (*mock.Server, string) {
	t.Helper()
	mockServer := mock.NewServer("127.0.0.1:18080")
	err := mockServer.Start()
	require.NoError(t, err)

	// Get the WebSocket URL
	endpoint := mockServer.URL()

	t.Cleanup(func() {
		mockServer.Stop()
	})

	return mockServer, endpoint
}

func TestNewClient(t *testing.T) {
	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: testEndpoint,
	}

	c := client.NewClient(cfg)
	assert.NotNil(t, c)
	assert.Equal(t, testEndpoint, c.Config().Endpoint)
	assert.NotEmpty(t, c.UserID())
}

func TestClientBuilder(t *testing.T) {
	c, err := client.NewClientBuilder().
		WithAPIKey("test-key").
		WithEndpoint("wss://example.com/ws").
		WithReconnectDelay(time.Second * 3).
		WithMaxReconnects(5).
		WithHeartbeatInterval(time.Second * 15).
		WithDebug(true).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, time.Second*3, c.Config().ReconnectDelay)
	assert.Equal(t, 5, c.Config().MaxReconnects)
}

func TestClientBuilderValidation(t *testing.T) {
	// Missing API key
	_, err := client.NewClientBuilder().
		WithEndpoint("wss://example.com/ws").
		Build()
	assert.Error(t, err)

	// Missing endpoint
	_, err = client.NewClientBuilder().
		WithAPIKey("test-key").
		Build()
	assert.Error(t, err)
}

func TestClientConnectDisconnect(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	// Connect
	err := c.Connect()
	assert.NoError(t, err)
	assert.True(t, c.IsConnected())

	// Disconnect
	err = c.Disconnect()
	assert.NoError(t, err)
	assert.False(t, c.IsConnected())
}

func TestClientClose(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	err := c.Connect()
	assert.NoError(t, err)

	err = c.Close()
	assert.NoError(t, err)

	// Should return error when trying to connect after close
	err = c.Connect()
	assert.Equal(t, client.ErrClosed, err)
}

func TestSendMessageNotConnected(t *testing.T) {
	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: testEndpoint,
	}

	c := client.NewClient(cfg)

	err := c.SendMessage("general", "Hello!")
	assert.Equal(t, client.ErrNotConnected, err)
}

func TestSendMessage(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	err = c.SendMessage("general", "Hello, world!")
	assert.NoError(t, err)
}

func TestSendMessageWithOptions(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	opts := message.Options{
		Priority: message.HighPriority,
		TTL:      time.Minute * 5,
		Metadata: map[string]interface{}{
			"custom": "data",
		},
	}

	err = c.SendMessageWithOptions("general", "Important message!", opts)
	assert.NoError(t, err)
}

func TestEventSubscription(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedEvent events.Event
	unsubscribe := c.On(events.Connected, func(e events.Event) {
		receivedEvent = e
		wg.Done()
	})
	defer unsubscribe()

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	// Wait for event
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, events.Connected, receivedEvent.Type)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for connected event")
	}
}

func TestUnsubscribe(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	callCount := 0
	unsubscribe := c.On(events.Connected, func(e events.Event) {
		callCount++
	})

	// Unsubscribe before connecting
	unsubscribe()

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	// Give time for any events to be processed
	time.Sleep(100 * time.Millisecond)

	// Handler should not have been called
	assert.Equal(t, 0, callCount)
}

func TestJoinLeaveChannel(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	// Join channel
	err = c.JoinChannel("general")
	assert.NoError(t, err)

	channels := c.Channels()
	assert.Contains(t, channels, "general")

	// Leave channel
	err = c.LeaveChannel("general")
	assert.NoError(t, err)

	channels = c.Channels()
	assert.NotContains(t, channels, "general")
}

func TestStream(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	err := c.Connect()
	require.NoError(t, err)
	defer c.Close()

	// Start stream
	stream, err := c.StartStream("test-stream")
	require.NoError(t, err)
	assert.True(t, stream.IsActive())

	// Send frame
	err = stream.SendFrame([]byte("frame-data"))
	assert.NoError(t, err)

	// End stream
	stream.End()
	assert.False(t, stream.IsActive())

	// Sending to ended stream should error
	err = stream.SendFrame([]byte("more-data"))
	assert.Error(t, err)
}

func TestContextCancellation(t *testing.T) {
	_, endpoint := setupMockServer(t)

	cfg := config.Config{
		APIKey:   testAPIKey,
		Endpoint: endpoint,
	}

	c := client.NewClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := c.ConnectWithContext(ctx)
	// Should return error (either context error or connection error due to cancellation)
	assert.Error(t, err)
}
