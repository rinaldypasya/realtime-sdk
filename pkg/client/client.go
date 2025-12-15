// Package client provides the main SDK client for realtime messaging.
package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/connection"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/handlers"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/factory"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/observer"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/strategy"
	"github.com/google/uuid"
)

// Common errors
var (
	ErrNotConnected     = errors.New("client: not connected")
	ErrAlreadyConnected = errors.New("client: already connected")
	ErrChannelNotFound  = errors.New("client: channel not found")
	ErrRateLimited      = errors.New("client: rate limited")
	ErrClosed           = errors.New("client: client is closed")
)

// Client is the main SDK client for realtime messaging.
type Client struct {
	mu              sync.RWMutex
	config          config.Config
	connManager     *connection.Manager
	eventBus        *observer.EventBus
	handlerRegistry *handlers.HandlerRegistry
	factory         *factory.RealtimeFactory
	userID          string
	channels        map[string]bool
	closed          bool
	closeChan       chan struct{}
}

// NewClient creates a new realtime client with the given configuration.
func NewClient(cfg config.Config) *Client {
	cfg = cfg.WithDefaults()

	userID := uuid.New().String()
	eventBus := observer.NewEventBus()
	connStrategy := strategy.NewWebSocketStrategy(cfg)
	connManager := connection.NewManager(cfg, connStrategy, eventBus)

	client := &Client{
		config:          cfg,
		connManager:     connManager,
		eventBus:        eventBus,
		handlerRegistry: handlers.NewHandlerRegistry(),
		factory:         factory.NewRealtimeFactory(userID),
		userID:          userID,
		channels:        make(map[string]bool),
		closeChan:       make(chan struct{}),
	}

	// Set up internal event handlers
	client.setupInternalHandlers()

	return client
}

// setupInternalHandlers sets up internal event handlers.
func (c *Client) setupInternalHandlers() {
	c.eventBus.Subscribe(events.MessageReceived, func(e events.Event) {
		if msg, ok := e.Payload.(*message.Message); ok {
			ctx := context.Background()
			c.handlerRegistry.HandleMessage(ctx, msg)
		}
	})
}

// Connect establishes a connection to the server.
func (c *Client) Connect() error {
	return c.ConnectWithContext(context.Background())
}

// ConnectWithContext establishes a connection with a context.
func (c *Client) ConnectWithContext(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	c.mu.Unlock()

	return c.connManager.Connect(ctx)
}

// Disconnect closes the connection but keeps the client reusable.
func (c *Client) Disconnect() error {
	return c.DisconnectWithContext(context.Background())
}

// DisconnectWithContext disconnects with a context.
func (c *Client) DisconnectWithContext(ctx context.Context) error {
	return c.connManager.Disconnect(ctx)
}

// Close permanently closes the client and releases all resources.
func (c *Client) Close() error {
	return c.CloseWithContext(context.Background())
}

// CloseWithContext permanently closes the client with a context.
func (c *Client) CloseWithContext(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeChan)
	c.mu.Unlock()

	// Clear handlers
	c.handlerRegistry.Clear()
	c.eventBus.Clear()

	// Close connection manager
	return c.connManager.Close(ctx)
}

// SendMessage sends a text message to a channel.
func (c *Client) SendMessage(channel, content string) error {
	return c.SendMessageWithContext(context.Background(), channel, content)
}

// SendMessageWithContext sends a message with a context.
func (c *Client) SendMessageWithContext(ctx context.Context, channel, content string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	msg := c.factory.Messages().CreateTextMessage(channel, content)
	return c.connManager.Send(ctx, msg)
}

// SendMessageWithOptions sends a message with additional options.
func (c *Client) SendMessageWithOptions(channel, content string, opts message.Options) error {
	return c.SendMessageWithOptionsContext(context.Background(), channel, content, opts)
}

// SendMessageWithOptionsContext sends a message with options and context.
func (c *Client) SendMessageWithOptionsContext(ctx context.Context, channel, content string, opts message.Options) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	msg := c.factory.Messages().CreateTextMessage(channel, content)
	msg.Priority = opts.Priority
	msg.TTL = opts.TTL
	msg.Metadata = opts.Metadata
	msg.Attachments = opts.Attachments
	msg.Mentions = opts.Mentions
	msg.ReplyTo = opts.ReplyTo

	return c.connManager.Send(ctx, msg)
}

// SendImage sends an image message to a channel.
func (c *Client) SendImage(channel, imageURL string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	msg := c.factory.Messages().CreateImageMessage(channel, imageURL)
	return c.connManager.Send(context.Background(), msg)
}

// SendFile sends a file message to a channel.
func (c *Client) SendFile(channel, fileName, fileURL string, fileSize int64) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	msg := c.factory.Messages().CreateFileMessage(channel, fileName, fileURL, fileSize)
	return c.connManager.Send(context.Background(), msg)
}

// JoinChannel joins a channel.
func (c *Client) JoinChannel(channelID string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	c.mu.Lock()
	c.channels[channelID] = true
	c.mu.Unlock()

	// Send join request to server
	return c.connManager.SendJoinChannel(context.Background(), channelID)
}

// LeaveChannel leaves a channel.
func (c *Client) LeaveChannel(channelID string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	c.mu.Lock()
	delete(c.channels, channelID)
	c.mu.Unlock()

	// Send leave request to server
	return c.connManager.SendLeaveChannel(context.Background(), channelID)
}

// On registers an event handler for a specific event type.
// Returns an unsubscribe function.
func (c *Client) On(eventType events.EventType, handler events.Handler) func() {
	return c.eventBus.Subscribe(eventType, handler)
}

// OnMessage registers a handler for messages on a specific channel.
func (c *Client) OnMessage(channel string, handler handlers.MessageHandler) {
	c.handlerRegistry.OnMessage(channel, handler)
}

// OnAllMessages registers a handler for all messages.
func (c *Client) OnAllMessages(handler handlers.MessageHandler) {
	c.handlerRegistry.OnAllMessages(handler)
}

// Use adds middleware to the message handler chain.
func (c *Client) Use(middleware handlers.Middleware) {
	c.handlerRegistry.Use(middleware)
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	return c.connManager.IsConnected()
}

// State returns the current connection state.
func (c *Client) State() connection.State {
	return c.connManager.State()
}

// UserID returns the client's user ID.
func (c *Client) UserID() string {
	return c.userID
}

// Config returns the client configuration.
func (c *Client) Config() config.Config {
	return c.config
}

// Channels returns the list of joined channels.
func (c *Client) Channels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	channels := make([]string, 0, len(c.channels))
	for ch := range c.channels {
		channels = append(channels, ch)
	}
	return channels
}

// Stream represents a video/audio stream.
type Stream struct {
	ID       string
	client   *Client
	active   bool
	mu       sync.Mutex
	sequence int64
}

// StartStream starts a new video/audio stream.
func (c *Client) StartStream(streamID string) (*Stream, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	stream := &Stream{
		ID:     streamID,
		client: c,
		active: true,
	}

	c.eventBus.Notify(events.NewEvent(events.StreamStarted, &events.StreamEvent{
		StreamID: streamID,
	}))

	return stream, nil
}

// SendFrame sends a video/audio frame.
func (s *Stream) SendFrame(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return errors.New("stream: stream is not active")
	}

	s.sequence++

	s.client.eventBus.Notify(events.NewEvent(events.FrameReceived, &events.StreamEvent{
		StreamID:  s.ID,
		FrameData: data,
	}))

	return nil
}

// End ends the stream.
func (s *Stream) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	s.client.eventBus.Notify(events.NewEvent(events.StreamEnded, &events.StreamEvent{
		StreamID: s.ID,
	}))
}

// IsActive returns true if the stream is active.
func (s *Stream) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Builder provides a fluent interface for building a Client.
type Builder struct {
	config config.Config
}

// NewClientBuilder creates a new client builder.
func NewClientBuilder() *Builder {
	return &Builder{
		config: config.Config{}.WithDefaults(),
	}
}

// WithAPIKey sets the API key.
func (b *Builder) WithAPIKey(key string) *Builder {
	b.config.APIKey = key
	return b
}

// WithEndpoint sets the WebSocket endpoint.
func (b *Builder) WithEndpoint(endpoint string) *Builder {
	b.config.Endpoint = endpoint
	return b
}

// WithUserID sets the user identifier.
func (b *Builder) WithUserID(userID string) *Builder {
	b.config.UserID = userID
	return b
}

// WithReconnectDelay sets the reconnection delay.
func (b *Builder) WithReconnectDelay(delay time.Duration) *Builder {
	b.config.ReconnectDelay = delay
	return b
}

// WithMaxReconnects sets the maximum reconnection attempts.
func (b *Builder) WithMaxReconnects(max int) *Builder {
	b.config.MaxReconnects = max
	return b
}

// WithHeartbeatInterval sets the heartbeat interval.
func (b *Builder) WithHeartbeatInterval(interval time.Duration) *Builder {
	b.config.HeartbeatInterval = interval
	return b
}

// WithDebug enables or disables debug mode.
func (b *Builder) WithDebug(enabled bool) *Builder {
	b.config.Debug = enabled
	return b
}

// Build creates the Client.
func (b *Builder) Build() (*Client, error) {
	if err := b.config.Validate(); err != nil {
		return nil, err
	}
	return NewClient(b.config), nil
}

// MustBuild creates the Client and panics on error.
func (b *Builder) MustBuild() *Client {
	client, err := b.Build()
	if err != nil {
		panic(err)
	}
	return client
}
