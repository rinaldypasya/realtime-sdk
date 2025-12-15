// Package mock provides a mock backend server for testing the realtime SDK.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Server is a mock WebSocket server for testing.
type Server struct {
	mu          sync.RWMutex
	addr        string
	server      *http.Server
	upgrader    websocket.Upgrader
	clients     map[string]*Client
	channels    map[string]map[string]*Client
	messages    []*message.Message
	onMessage   func(*message.Message)
	onConnect   func(*Client)
	onDisconnect func(*Client)
	started     bool
}

// Client represents a connected client.
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Channels []string
	mu       sync.Mutex
}

// NewServer creates a new mock server.
func NewServer(addr string) *Server {
	return &Server{
		addr:     addr,
		clients:  make(map[string]*Client),
		channels: make(map[string]map[string]*Client),
		messages: make([]*message.Message, 0),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Start starts the mock server.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/realtime", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the mock server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Close all client connections
	for _, client := range s.clients {
		client.Conn.Close()
	}

	s.started = false
	return s.server.Shutdown(ctx)
}

// handleHealth handles health check requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleWebSocket handles WebSocket connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	apiKey := r.URL.Query().Get("api_key")
	if apiKey == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		ID:       uuid.New().String(),
		Conn:     conn,
		Channels: make([]string, 0),
	}

	s.mu.Lock()
	s.clients[client.ID] = client
	s.mu.Unlock()

	if s.onConnect != nil {
		s.onConnect(client)
	}

	// Send connected event
	s.sendToClient(client, &message.Envelope{
		Action:    "connected",
		Timestamp: time.Now(),
	})

	go s.handleClient(client)
}

// handleClient handles messages from a client.
func (s *Server) handleClient(client *Client) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, client.ID)
		// Remove from all channels
		for _, channelID := range client.Channels {
			if channel, ok := s.channels[channelID]; ok {
				delete(channel, client.ID)
			}
		}
		s.mu.Unlock()

		if s.onDisconnect != nil {
			s.onDisconnect(client)
		}
		client.Conn.Close()
	}()

	for {
		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope message.Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}

		s.handleEnvelope(client, &envelope)
	}
}

// handleEnvelope processes an incoming envelope.
func (s *Server) handleEnvelope(client *Client, envelope *message.Envelope) {
	switch envelope.Action {
	case message.ActionSend:
		s.handleSendMessage(client, envelope)
	case message.ActionJoin:
		s.handleJoinChannel(client, envelope)
	case message.ActionLeave:
		s.handleLeaveChannel(client, envelope)
	case message.ActionTyping:
		s.handleTyping(client, envelope)
	case message.ActionHeartbeat:
		s.handleHeartbeat(client, envelope)
	}
}

// handleSendMessage handles a send message request.
func (s *Server) handleSendMessage(client *Client, envelope *message.Envelope) {
	msg := envelope.Message
	if msg == nil {
		return
	}

	msg.ID = uuid.New().String()
	msg.Timestamp = time.Now()
	msg.Status = message.StatusDelivered

	s.mu.Lock()
	s.messages = append(s.messages, msg)
	s.mu.Unlock()

	if s.onMessage != nil {
		s.onMessage(msg)
	}

	// Send acknowledgment
	s.sendToClient(client, &message.Envelope{
		Action:    message.ActionAck,
		Message:   msg,
		Timestamp: time.Now(),
		RequestID: envelope.RequestID,
	})

	// Broadcast to channel
	s.broadcastToChannel(msg.Channel, &message.Envelope{
		Action:    message.ActionReceive,
		Message:   msg,
		Timestamp: time.Now(),
	}, client.ID)
}

// handleJoinChannel handles a channel join request.
func (s *Server) handleJoinChannel(client *Client, envelope *message.Envelope) {
	channelID := envelope.Channel

	s.mu.Lock()
	if s.channels[channelID] == nil {
		s.channels[channelID] = make(map[string]*Client)
	}
	s.channels[channelID][client.ID] = client
	client.Channels = append(client.Channels, channelID)
	s.mu.Unlock()

	// Notify client
	s.sendToClient(client, &message.Envelope{
		Action:    message.ActionAck,
		Channel:   channelID,
		Timestamp: time.Now(),
		RequestID: envelope.RequestID,
	})

	// Notify channel members
	s.broadcastToChannel(channelID, &message.Envelope{
		Action:    "user_joined",
		Channel:   channelID,
		Timestamp: time.Now(),
		Message: &message.Message{
			Type:     message.TypeSystem,
			Content:  fmt.Sprintf("User %s joined", client.ID),
			SenderID: client.ID,
		},
	}, "")
}

// handleLeaveChannel handles a channel leave request.
func (s *Server) handleLeaveChannel(client *Client, envelope *message.Envelope) {
	channelID := envelope.Channel

	s.mu.Lock()
	if channel, ok := s.channels[channelID]; ok {
		delete(channel, client.ID)
	}
	// Remove from client's channels
	for i, ch := range client.Channels {
		if ch == channelID {
			client.Channels = append(client.Channels[:i], client.Channels[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	// Notify channel members
	s.broadcastToChannel(channelID, &message.Envelope{
		Action:    "user_left",
		Channel:   channelID,
		Timestamp: time.Now(),
		Message: &message.Message{
			Type:     message.TypeSystem,
			Content:  fmt.Sprintf("User %s left", client.ID),
			SenderID: client.ID,
		},
	}, "")
}

// handleTyping handles a typing indicator.
func (s *Server) handleTyping(client *Client, envelope *message.Envelope) {
	s.broadcastToChannel(envelope.Channel, &message.Envelope{
		Action:    message.ActionTyping,
		Channel:   envelope.Channel,
		Timestamp: time.Now(),
		Message: &message.Message{
			SenderID: client.ID,
		},
	}, client.ID)
}

// handleHeartbeat handles a heartbeat ping.
func (s *Server) handleHeartbeat(client *Client, envelope *message.Envelope) {
	s.sendToClient(client, &message.Envelope{
		Action:    message.ActionHeartbeat,
		Timestamp: time.Now(),
		RequestID: envelope.RequestID,
	})
}

// sendToClient sends an envelope to a specific client.
func (s *Server) sendToClient(client *Client, envelope *message.Envelope) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	return client.Conn.WriteMessage(websocket.TextMessage, data)
}

// broadcastToChannel sends an envelope to all clients in a channel.
func (s *Server) broadcastToChannel(channelID string, envelope *message.Envelope, excludeClientID string) {
	s.mu.RLock()
	channel := s.channels[channelID]
	s.mu.RUnlock()

	if channel == nil {
		return
	}

	for clientID, client := range channel {
		if clientID != excludeClientID {
			s.sendToClient(client, envelope)
		}
	}
}

// BroadcastMessage sends a message to all clients in a channel.
func (s *Server) BroadcastMessage(channelID string, msg *message.Message) {
	msg.ID = uuid.New().String()
	msg.Timestamp = time.Now()

	s.broadcastToChannel(channelID, &message.Envelope{
		Action:    message.ActionReceive,
		Message:   msg,
		Timestamp: time.Now(),
	}, "")
}

// SimulateEvent simulates an event to a specific client.
func (s *Server) SimulateEvent(clientID string, eventType events.EventType, payload interface{}) error {
	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	return s.sendToClient(client, &message.Envelope{
		Action:    string(eventType),
		Timestamp: time.Now(),
	})
}

// OnMessage sets a callback for incoming messages.
func (s *Server) OnMessage(handler func(*message.Message)) {
	s.onMessage = handler
}

// OnConnect sets a callback for new connections.
func (s *Server) OnConnect(handler func(*Client)) {
	s.onConnect = handler
}

// OnDisconnect sets a callback for disconnections.
func (s *Server) OnDisconnect(handler func(*Client)) {
	s.onDisconnect = handler
}

// GetMessages returns all received messages.
func (s *Server) GetMessages() []*message.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*message.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// GetClients returns all connected clients.
func (s *Server) GetClients() []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		result = append(result, client)
	}
	return result
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// Address returns the server address.
func (s *Server) Address() string {
	return s.addr
}

// URL returns the WebSocket URL for the server.
func (s *Server) URL() string {
	return fmt.Sprintf("ws://%s/realtime", s.addr)
}
