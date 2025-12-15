// Package strategy implements the Strategy design pattern for connection handling.
package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/gorilla/websocket"
)

// Common errors
var (
	ErrNotConnected    = errors.New("strategy: not connected")
	ErrAlreadyConnected = errors.New("strategy: already connected")
	ErrConnectionFailed = errors.New("strategy: connection failed")
	ErrSendFailed       = errors.New("strategy: send failed")
)

// ConnectionStrategy defines the interface for different connection strategies.
type ConnectionStrategy interface {
	// Connect establishes a connection to the server
	Connect(ctx context.Context) error

	// Disconnect closes the connection
	Disconnect(ctx context.Context) error

	// Send sends a message through the connection
	Send(ctx context.Context, msg *message.Message) error

	// SendJoinChannel sends a join channel request
	SendJoinChannel(ctx context.Context, channelID string) error

	// SendLeaveChannel sends a leave channel request
	SendLeaveChannel(ctx context.Context, channelID string) error

	// Receive returns a channel for receiving messages
	Receive() <-chan *message.Message

	// Events returns a channel for connection events
	Events() <-chan events.Event

	// IsConnected returns true if connected
	IsConnected() bool

	// Type returns the strategy type name
	Type() string
}

// WebSocketStrategy implements ConnectionStrategy for WebSocket connections.
type WebSocketStrategy struct {
	mu          sync.RWMutex
	config      config.Config
	conn        *websocket.Conn
	connected   bool
	msgChan     chan *message.Message
	eventChan   chan events.Event
	closeChan   chan struct{}
	sendQueue   chan *message.Message
}

// Envelope wraps messages for transmission (matching server protocol)
type Envelope struct {
	Action    string           `json:"action"`
	Message   *message.Message `json:"message,omitempty"`
	Channel   string           `json:"channel,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
	RequestID string           `json:"request_id,omitempty"`
	Error     *ErrorInfo       `json:"error,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewWebSocketStrategy creates a new WebSocket connection strategy.
func NewWebSocketStrategy(cfg config.Config) *WebSocketStrategy {
	return &WebSocketStrategy{
		config:    cfg,
		msgChan:   make(chan *message.Message, 100),
		eventChan: make(chan events.Event, 100),
		closeChan: make(chan struct{}),
		sendQueue: make(chan *message.Message, 100),
	}
}

// Connect establishes a WebSocket connection.
func (ws *WebSocketStrategy) Connect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.connected {
		return ErrAlreadyConnected
	}

	// Parse endpoint URL and add query parameters
	u, err := url.Parse(ws.config.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Add API key and username as query parameters
	q := u.Query()
	q.Set("api_key", ws.config.APIKey)
	if ws.config.UserID != "" {
		q.Set("username", ws.config.UserID)
	}
	u.RawQuery = q.Encode()

	// Create WebSocket connection
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	ws.conn = conn
	ws.connected = true
	ws.closeChan = make(chan struct{})

	// Start read/write loops
	go ws.readLoop()
	go ws.writeLoop()

	return nil
}

// readLoop reads messages from the WebSocket connection.
func (ws *WebSocketStrategy) readLoop() {
	defer func() {
		ws.mu.Lock()
		if ws.connected {
			ws.connected = false
			ws.eventChan <- events.NewEvent(events.ConnectionLost, nil)
		}
		ws.mu.Unlock()
	}()

	for {
		select {
		case <-ws.closeChan:
			return
		default:
			ws.mu.RLock()
			conn := ws.conn
			ws.mu.RUnlock()

			if conn == nil {
				return
			}

			// Read message from WebSocket
			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					ws.eventChan <- events.NewEventWithError(events.ConnectionError, err)
				}
				return
			}

			// Parse envelope
			var envelope Envelope
			if err := json.Unmarshal(data, &envelope); err != nil {
				// Ignore malformed messages
				continue
			}

			// Handle different actions
			switch envelope.Action {
			case "message_received":
				if envelope.Message != nil {
					ws.msgChan <- envelope.Message
				}
			case "ack":
				// Message sent acknowledgment
				if envelope.Message != nil {
					ws.eventChan <- events.NewEvent(events.MessageSent, &events.MessageEvent{
						MessageID: envelope.Message.ID,
						Channel:   envelope.Message.Channel,
						Content:   envelope.Message.Content,
					})
				}
			case "channel_joined":
				ws.eventChan <- events.NewEvent(events.ChannelJoined, envelope.Channel)
			case "channel_left":
				ws.eventChan <- events.NewEvent(events.ChannelLeft, envelope.Channel)
			case "user_joined":
				if envelope.Message != nil {
					ws.eventChan <- events.NewEvent(events.UserJoined, &events.PresenceEvent{
						UserID:   envelope.Message.SenderID,
						Username: envelope.Message.SenderID,
						Channel:  envelope.Channel,
					})
				}
			case "user_left":
				if envelope.Message != nil {
					ws.eventChan <- events.NewEvent(events.UserLeft, &events.PresenceEvent{
						UserID:   envelope.Message.SenderID,
						Username: envelope.Message.SenderID,
						Channel:  envelope.Channel,
					})
				}
			case "user_typing":
				if envelope.Message != nil {
					ws.eventChan <- events.NewEvent(events.UserTyping, &events.PresenceEvent{
						UserID:   envelope.Message.SenderID,
						Username: envelope.Message.SenderID,
						Channel:  envelope.Channel,
					})
				}
			case "connected":
				ws.eventChan <- events.NewEvent(events.Connected, &events.ConnectionEvent{
					Endpoint: ws.config.Endpoint,
				})
			case "heartbeat":
				// Heartbeat response - ignore or handle if needed
			}
		}
	}
}

// writeLoop processes the send queue.
func (ws *WebSocketStrategy) writeLoop() {
	for {
		select {
		case <-ws.closeChan:
			return
		case msg := <-ws.sendQueue:
			ws.mu.RLock()
			conn := ws.conn
			ws.mu.RUnlock()

			if conn == nil {
				continue
			}

			// Create envelope for sending
			envelope := Envelope{
				Action:    "send",
				Message:   msg,
				Channel:   msg.Channel,
				Timestamp: time.Now(),
				RequestID: msg.ID,
			}

			// Marshal to JSON
			data, err := json.Marshal(envelope)
			if err != nil {
				ws.eventChan <- events.NewEventWithError(events.ConnectionError, err)
				continue
			}

			// Write to WebSocket
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				ws.eventChan <- events.NewEventWithError(events.ConnectionError, err)
				// Connection lost, readLoop will handle reconnection
				return
			}
		}
	}
}

// Disconnect closes the WebSocket connection.
func (ws *WebSocketStrategy) Disconnect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.connected {
		return nil
	}

	close(ws.closeChan)
	ws.connected = false

	// Close WebSocket connection
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}

	ws.eventChan <- events.NewEvent(events.Disconnected, nil)

	return nil
}

// Send sends a message through the WebSocket.
func (ws *WebSocketStrategy) Send(ctx context.Context, msg *message.Message) error {
	ws.mu.RLock()
	connected := ws.connected
	ws.mu.RUnlock()

	if !connected {
		return ErrNotConnected
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ws.sendQueue <- msg:
		return nil
	default:
		return ErrSendFailed
	}
}

// SendJoinChannel sends a join channel request to the server.
func (ws *WebSocketStrategy) SendJoinChannel(ctx context.Context, channelID string) error {
	ws.mu.RLock()
	conn := ws.conn
	connected := ws.connected
	ws.mu.RUnlock()

	if !connected || conn == nil {
		return ErrNotConnected
	}

	envelope := Envelope{
		Action:    "join",
		Channel:   channelID,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// SendLeaveChannel sends a leave channel request to the server.
func (ws *WebSocketStrategy) SendLeaveChannel(ctx context.Context, channelID string) error {
	ws.mu.RLock()
	conn := ws.conn
	connected := ws.connected
	ws.mu.RUnlock()

	if !connected || conn == nil {
		return ErrNotConnected
	}

	envelope := Envelope{
		Action:    "leave",
		Channel:   channelID,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// Receive returns the message receive channel.
func (ws *WebSocketStrategy) Receive() <-chan *message.Message {
	return ws.msgChan
}

// Events returns the events channel.
func (ws *WebSocketStrategy) Events() <-chan events.Event {
	return ws.eventChan
}

// IsConnected returns the connection status.
func (ws *WebSocketStrategy) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.connected
}

// Type returns the strategy type.
func (ws *WebSocketStrategy) Type() string {
	return "websocket"
}

// SimulateIncomingMessage simulates receiving a message (for testing).
func (ws *WebSocketStrategy) SimulateIncomingMessage(msg *message.Message) {
	ws.msgChan <- msg
}

// HTTPPollingStrategy implements ConnectionStrategy for HTTP long-polling.
type HTTPPollingStrategy struct {
	mu           sync.RWMutex
	config       config.Config
	connected    bool
	msgChan      chan *message.Message
	eventChan    chan events.Event
	closeChan    chan struct{}
	pollInterval time.Duration
}

// NewHTTPPollingStrategy creates a new HTTP polling strategy.
func NewHTTPPollingStrategy(cfg config.Config, pollInterval time.Duration) *HTTPPollingStrategy {
	return &HTTPPollingStrategy{
		config:       cfg,
		msgChan:      make(chan *message.Message, 100),
		eventChan:    make(chan events.Event, 100),
		pollInterval: pollInterval,
	}
}

// Connect starts the HTTP polling.
func (hp *HTTPPollingStrategy) Connect(ctx context.Context) error {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if hp.connected {
		return ErrAlreadyConnected
	}

	hp.connected = true
	hp.closeChan = make(chan struct{})

	go hp.pollLoop()

	hp.eventChan <- events.NewEvent(events.Connected, &events.ConnectionEvent{
		Endpoint: hp.config.Endpoint,
	})

	return nil
}

// pollLoop performs periodic HTTP polling.
func (hp *HTTPPollingStrategy) pollLoop() {
	ticker := time.NewTicker(hp.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hp.closeChan:
			return
		case <-ticker.C:
			// In real implementation, make HTTP request to poll for messages
		}
	}
}

// Disconnect stops the HTTP polling.
func (hp *HTTPPollingStrategy) Disconnect(ctx context.Context) error {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if !hp.connected {
		return nil
	}

	close(hp.closeChan)
	hp.connected = false

	hp.eventChan <- events.NewEvent(events.Disconnected, nil)

	return nil
}

// Send sends a message via HTTP POST.
func (hp *HTTPPollingStrategy) Send(ctx context.Context, msg *message.Message) error {
	hp.mu.RLock()
	connected := hp.connected
	hp.mu.RUnlock()

	if !connected {
		return ErrNotConnected
	}

	// In real implementation, make HTTP POST request
	hp.eventChan <- events.NewEvent(events.MessageSent, &events.MessageEvent{
		MessageID: msg.ID,
		Channel:   msg.Channel,
		Content:   msg.Content,
	})

	return nil
}

// SendJoinChannel sends a join channel request via HTTP.
func (hp *HTTPPollingStrategy) SendJoinChannel(ctx context.Context, channelID string) error {
	hp.mu.RLock()
	connected := hp.connected
	hp.mu.RUnlock()

	if !connected {
		return ErrNotConnected
	}

	// In real implementation, make HTTP POST request
	return nil
}

// SendLeaveChannel sends a leave channel request via HTTP.
func (hp *HTTPPollingStrategy) SendLeaveChannel(ctx context.Context, channelID string) error {
	hp.mu.RLock()
	connected := hp.connected
	hp.mu.RUnlock()

	if !connected {
		return ErrNotConnected
	}

	// In real implementation, make HTTP POST request
	return nil
}

// Receive returns the message receive channel.
func (hp *HTTPPollingStrategy) Receive() <-chan *message.Message {
	return hp.msgChan
}

// Events returns the events channel.
func (hp *HTTPPollingStrategy) Events() <-chan events.Event {
	return hp.eventChan
}

// IsConnected returns the connection status.
func (hp *HTTPPollingStrategy) IsConnected() bool {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	return hp.connected
}

// Type returns the strategy type.
func (hp *HTTPPollingStrategy) Type() string {
	return "http_polling"
}

// StrategySelector helps select the appropriate connection strategy.
type StrategySelector struct {
	strategies map[string]ConnectionStrategy
	current    ConnectionStrategy
	mu         sync.RWMutex
}

// NewStrategySelector creates a new strategy selector.
func NewStrategySelector() *StrategySelector {
	return &StrategySelector{
		strategies: make(map[string]ConnectionStrategy),
	}
}

// Register adds a strategy to the selector.
func (ss *StrategySelector) Register(name string, strategy ConnectionStrategy) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.strategies[name] = strategy
}

// Select chooses a strategy by name.
func (ss *StrategySelector) Select(name string) (ConnectionStrategy, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	strategy, exists := ss.strategies[name]
	if !exists {
		return nil, errors.New("strategy not found: " + name)
	}

	ss.current = strategy
	return strategy, nil
}

// Current returns the currently selected strategy.
func (ss *StrategySelector) Current() ConnectionStrategy {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.current
}
