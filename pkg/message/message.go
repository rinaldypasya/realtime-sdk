// Package message defines message types and structures for the realtime SDK.
package message

import (
	"encoding/json"
	"time"
)

// Type represents the type of message.
type Type string

// Message types
const (
	TypeText    Type = "text"
	TypeImage   Type = "image"
	TypeVideo   Type = "video"
	TypeAudio   Type = "audio"
	TypeFile    Type = "file"
	TypeSystem  Type = "system"
	TypeTyping  Type = "typing"
	TypeReaction Type = "reaction"
)

// Status represents the delivery status of a message.
type Status string

// Message statuses
const (
	StatusPending   Status = "pending"
	StatusSent      Status = "sent"
	StatusDelivered Status = "delivered"
	StatusRead      Status = "read"
	StatusFailed    Status = "failed"
)

// Priority represents message priority.
type Priority int

// Priority levels
const (
	LowPriority    Priority = 0
	NormalPriority Priority = 1
	HighPriority   Priority = 2
	UrgentPriority Priority = 3
)

// Message represents a chat message.
type Message struct {
	// ID is the unique identifier for the message
	ID string `json:"id"`

	// Type is the message type
	Type Type `json:"type"`

	// Channel is the channel/room the message belongs to
	Channel string `json:"channel"`

	// Content is the message content
	Content string `json:"content"`

	// SenderID is the ID of the message sender
	SenderID string `json:"sender_id"`

	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`

	// Status is the delivery status
	Status Status `json:"status"`

	// Priority is the message priority
	Priority Priority `json:"priority,omitempty"`

	// ReplyTo is the ID of the message being replied to
	ReplyTo string `json:"reply_to,omitempty"`

	// Metadata contains additional message data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Attachments contains file attachments
	Attachments []Attachment `json:"attachments,omitempty"`

	// Mentions contains mentioned user IDs
	Mentions []string `json:"mentions,omitempty"`

	// EditedAt is when the message was last edited
	EditedAt *time.Time `json:"edited_at,omitempty"`

	// DeletedAt is when the message was deleted
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// TTL is the time-to-live for ephemeral messages
	TTL time.Duration `json:"ttl,omitempty"`
}

// Attachment represents a file attachment.
type Attachment struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// Options contains optional message parameters.
type Options struct {
	Priority    Priority
	TTL         time.Duration
	Metadata    map[string]interface{}
	Attachments []Attachment
	Mentions    []string
	ReplyTo     string
}

// ToJSON serializes the message to JSON.
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON deserializes a message from JSON.
func FromJSON(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// IsEdited returns true if the message has been edited.
func (m *Message) IsEdited() bool {
	return m.EditedAt != nil
}

// IsDeleted returns true if the message has been deleted.
func (m *Message) IsDeleted() bool {
	return m.DeletedAt != nil
}

// IsExpired returns true if the message has expired (for ephemeral messages).
func (m *Message) IsExpired() bool {
	if m.TTL == 0 {
		return false
	}
	return time.Since(m.Timestamp) > m.TTL
}

// HasAttachments returns true if the message has attachments.
func (m *Message) HasAttachments() bool {
	return len(m.Attachments) > 0
}

// Clone creates a copy of the message.
func (m *Message) Clone() *Message {
	clone := *m
	if m.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range m.Metadata {
			clone.Metadata[k] = v
		}
	}
	if m.Attachments != nil {
		clone.Attachments = make([]Attachment, len(m.Attachments))
		copy(clone.Attachments, m.Attachments)
	}
	if m.Mentions != nil {
		clone.Mentions = make([]string, len(m.Mentions))
		copy(clone.Mentions, m.Mentions)
	}
	return &clone
}

// Builder provides a fluent interface for building messages.
type Builder struct {
	message *Message
}

// NewBuilder creates a new message builder.
func NewBuilder() *Builder {
	return &Builder{
		message: &Message{
			Type:      TypeText,
			Status:    StatusPending,
			Priority:  NormalPriority,
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithID sets the message ID.
func (b *Builder) WithID(id string) *Builder {
	b.message.ID = id
	return b
}

// WithType sets the message type.
func (b *Builder) WithType(t Type) *Builder {
	b.message.Type = t
	return b
}

// WithChannel sets the channel.
func (b *Builder) WithChannel(channel string) *Builder {
	b.message.Channel = channel
	return b
}

// WithContent sets the content.
func (b *Builder) WithContent(content string) *Builder {
	b.message.Content = content
	return b
}

// WithSenderID sets the sender ID.
func (b *Builder) WithSenderID(senderID string) *Builder {
	b.message.SenderID = senderID
	return b
}

// WithPriority sets the priority.
func (b *Builder) WithPriority(priority Priority) *Builder {
	b.message.Priority = priority
	return b
}

// WithReplyTo sets the reply-to message ID.
func (b *Builder) WithReplyTo(replyTo string) *Builder {
	b.message.ReplyTo = replyTo
	return b
}

// WithMetadata adds metadata.
func (b *Builder) WithMetadata(key string, value interface{}) *Builder {
	b.message.Metadata[key] = value
	return b
}

// WithAttachment adds an attachment.
func (b *Builder) WithAttachment(attachment Attachment) *Builder {
	b.message.Attachments = append(b.message.Attachments, attachment)
	return b
}

// WithMention adds a mention.
func (b *Builder) WithMention(userID string) *Builder {
	b.message.Mentions = append(b.message.Mentions, userID)
	return b
}

// WithTTL sets the time-to-live.
func (b *Builder) WithTTL(ttl time.Duration) *Builder {
	b.message.TTL = ttl
	return b
}

// Build creates the message.
func (b *Builder) Build() *Message {
	return b.message
}

// Envelope wraps a message for transmission.
type Envelope struct {
	Action    string      `json:"action"`
	Message   *Message    `json:"message,omitempty"`
	Channel   string      `json:"channel,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo contains error details.
type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Actions for message envelopes
const (
	ActionSend      = "send"
	ActionReceive   = "receive"
	ActionAck       = "ack"
	ActionError     = "error"
	ActionJoin      = "join"
	ActionLeave     = "leave"
	ActionTyping    = "typing"
	ActionPresence  = "presence"
	ActionHeartbeat = "heartbeat"
)
