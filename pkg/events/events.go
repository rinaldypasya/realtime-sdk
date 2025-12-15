// Package events provides the event system for the realtime SDK.
package events

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event.
type EventType string

// Event types
const (
	// Connection events
	Connected       EventType = "connected"
	Disconnected    EventType = "disconnected"
	ConnectionLost  EventType = "connection_lost"
	Reconnecting    EventType = "reconnecting"
	Reconnected     EventType = "reconnected"
	ConnectionError EventType = "connection_error"

	// Message events
	MessageReceived  EventType = "message_received"
	MessageSent      EventType = "message_sent"
	MessageDelivered EventType = "message_delivered"
	MessageRead      EventType = "message_read"
	MessageError     EventType = "message_error"

	// Channel events
	ChannelJoined EventType = "channel_joined"
	ChannelLeft   EventType = "channel_left"
	ChannelError  EventType = "channel_error"

	// Presence events
	UserJoined  EventType = "user_joined"
	UserLeft    EventType = "user_left"
	UserTyping  EventType = "user_typing"
	UserOnline  EventType = "user_online"
	UserOffline EventType = "user_offline"

	// Stream events
	StreamStarted EventType = "stream_started"
	StreamEnded   EventType = "stream_ended"
	StreamError   EventType = "stream_error"
	FrameReceived EventType = "frame_received"

	// System events
	Heartbeat    EventType = "heartbeat"
	RateLimited  EventType = "rate_limited"
	ServerError  EventType = "server_error"
	ClientError  EventType = "client_error"
)

// Event represents an event in the system.
type Event struct {
	// ID is the unique identifier for this event
	ID string

	// Type is the event type
	Type EventType

	// Timestamp is when the event occurred
	Timestamp time.Time

	// Payload is the event data
	Payload interface{}

	// Metadata contains additional event information
	Metadata map[string]interface{}

	// Error contains any error associated with the event
	Error error
}

// NewEvent creates a new event with the given type and payload.
func NewEvent(eventType EventType, payload interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  make(map[string]interface{}),
	}
}

// NewEventWithError creates a new event with an error.
func NewEventWithError(eventType EventType, err error) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Error:     err,
		Metadata:  make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to the event.
func (e Event) WithMetadata(key string, value interface{}) Event {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// IsError returns true if the event represents an error.
func (e Event) IsError() bool {
	return e.Error != nil
}

// Handler is a function that handles events.
type Handler func(Event)

// Subscription represents a subscription to events.
type Subscription struct {
	ID        string
	EventType EventType
	Handler   Handler
}

// ConnectionEvent contains connection-related event data.
type ConnectionEvent struct {
	Endpoint       string
	ReconnectCount int
	Latency        time.Duration
}

// MessageEvent contains message-related event data.
type MessageEvent struct {
	MessageID string
	Channel   string
	Content   string
	SenderID  string
	Timestamp time.Time
}

// PresenceEvent contains presence-related event data.
type PresenceEvent struct {
	UserID    string
	Username  string
	Channel   string
	Status    string
	Timestamp time.Time
}

// StreamEvent contains stream-related event data.
type StreamEvent struct {
	StreamID  string
	Type      string
	Bitrate   int
	FrameData []byte
}

// ErrorEvent contains error-related event data.
type ErrorEvent struct {
	Code    int
	Message string
	Details map[string]interface{}
}
