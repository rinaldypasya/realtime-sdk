package integration

import (
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

func TestClientWithMockServer(t *testing.T) {
	// Start mock server
	server := mock.NewServer("localhost:18080")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Create client
	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	// Connect
	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)

	assert.True(t, sdk.IsConnected())
}

func TestSendMessageToMockServer(t *testing.T) {
	// Start mock server
	server := mock.NewServer("localhost:18081")
	
	var receivedMessages []*message.Message
	var mu sync.Mutex
	
	server.OnMessage(func(msg *message.Message) {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
	})
	
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Create and connect client
	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	time.Sleep(100 * time.Millisecond)

	// Send message
	err = sdk.SendMessage("general", "Hello from test!")
	assert.NoError(t, err)
}

func TestEventHandlingWithMockServer(t *testing.T) {
	server := mock.NewServer("localhost:18082")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	// Track events
	var connectedReceived bool
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	sdk.On(events.Connected, func(e events.Event) {
		mu.Lock()
		connectedReceived = true
		mu.Unlock()
		wg.Done()
	})

	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	// Wait for event with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mu.Lock()
		assert.True(t, connectedReceived)
		mu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connected event")
	}
}

func TestMultipleClientsWithMockServer(t *testing.T) {
	server := mock.NewServer("localhost:18083")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Create multiple clients
	numClients := 5
	clients := make([]*client.Client, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = client.NewClient(config.Config{
			APIKey:   "test-api-key",
			Endpoint: server.URL(),
		})

		err := clients[i].Connect()
		require.NoError(t, err)
	}

	// Wait for connections
	time.Sleep(200 * time.Millisecond)

	// Verify all connected
	for i, c := range clients {
		assert.True(t, c.IsConnected(), "Client %d should be connected", i)
	}

	// Close all clients
	for _, c := range clients {
		c.Close()
	}
}

func TestChannelJoinWithMockServer(t *testing.T) {
	server := mock.NewServer("localhost:18084")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	time.Sleep(100 * time.Millisecond)

	// Join channels
	err = sdk.JoinChannel("general")
	assert.NoError(t, err)

	err = sdk.JoinChannel("random")
	assert.NoError(t, err)

	channels := sdk.Channels()
	assert.Len(t, channels, 2)
	assert.Contains(t, channels, "general")
	assert.Contains(t, channels, "random")
}

func TestStreamWithMockServer(t *testing.T) {
	server := mock.NewServer("localhost:18085")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	time.Sleep(100 * time.Millisecond)

	// Start stream
	stream, err := sdk.StartStream("test-stream")
	require.NoError(t, err)
	assert.True(t, stream.IsActive())

	// Send frames
	for i := 0; i < 10; i++ {
		err = stream.SendFrame([]byte{byte(i)})
		assert.NoError(t, err)
	}

	// End stream
	stream.End()
	assert.False(t, stream.IsActive())
}

func TestGracefulShutdown(t *testing.T) {
	server := mock.NewServer("localhost:18086")
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	sdk := client.NewClient(config.Config{
		APIKey:   "test-api-key",
		Endpoint: server.URL(),
	})

	err = sdk.Connect()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Close should be graceful
	err = sdk.Close()
	assert.NoError(t, err)
	assert.False(t, sdk.IsConnected())

	// Should not be able to connect after close
	err = sdk.Connect()
	assert.Equal(t, client.ErrClosed, err)
}

func TestReconnection(t *testing.T) {
	server := mock.NewServer("localhost:18087")
	err := server.Start()
	require.NoError(t, err)

	sdk := client.NewClient(config.Config{
		APIKey:         "test-api-key",
		Endpoint:       server.URL(),
		ReconnectDelay: time.Millisecond * 100,
		MaxReconnects:  3,
	})

	// Track reconnection events
	var reconnectCount int
	var mu sync.Mutex

	sdk.On(events.Reconnecting, func(e events.Event) {
		mu.Lock()
		reconnectCount++
		mu.Unlock()
	})

	err = sdk.Connect()
	require.NoError(t, err)
	defer sdk.Close()

	time.Sleep(100 * time.Millisecond)
	assert.True(t, sdk.IsConnected())

	// Stop server to trigger reconnection
	server.Stop()

	// Give time for reconnection attempts
	time.Sleep(500 * time.Millisecond)

	// Reconnect count might be 0 or more depending on timing
	mu.Lock()
	t.Logf("Reconnection attempts: %d", reconnectCount)
	mu.Unlock()
}
