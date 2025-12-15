// Package protocol defines the wire protocol for realtime communication.
package protocol

import (
	"encoding/json"
	"time"
)

// Version is the current protocol version.
const Version = "1.0"

// OpCode represents the operation code for a frame.
type OpCode int

const (
	OpConnect OpCode = iota
	OpDisconnect
	OpMessage
	OpAck
	OpError
	OpPing
	OpPong
	OpSubscribe
	OpUnsubscribe
	OpPresence
	OpTyping
	OpStream
)

// Frame represents a protocol frame.
type Frame struct {
	Version   string          `json:"v"`
	OpCode    OpCode          `json:"op"`
	Sequence  int64           `json:"seq,omitempty"`
	Timestamp time.Time       `json:"ts"`
	Payload   json.RawMessage `json:"p,omitempty"`
	Error     *ProtocolError  `json:"e,omitempty"`
}

// ProtocolError represents a protocol-level error.
type ProtocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error codes
const (
	ErrCodeInvalidFrame     = 1000
	ErrCodeAuthFailed       = 1001
	ErrCodeRateLimited      = 1002
	ErrCodeChannelNotFound  = 1003
	ErrCodePermissionDenied = 1004
	ErrCodeServerError      = 1005
)

// ConnectPayload is the payload for connect frames.
type ConnectPayload struct {
	APIKey      string            `json:"api_key"`
	UserID      string            `json:"user_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Resume      bool              `json:"resume,omitempty"`
	LastSeq     int64             `json:"last_seq,omitempty"`
}

// ConnectAckPayload is the response for connect frames.
type ConnectAckPayload struct {
	SessionID       string `json:"session_id"`
	ServerTime      int64  `json:"server_time"`
	HeartbeatMs     int    `json:"heartbeat_ms"`
	ResumeSupported bool   `json:"resume_supported"`
}

// MessagePayload is the payload for message frames.
type MessagePayload struct {
	ID        string                 `json:"id"`
	Channel   string                 `json:"channel"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	SenderID  string                 `json:"sender_id"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// SubscribePayload is the payload for subscribe frames.
type SubscribePayload struct {
	Channels []string `json:"channels"`
}

// PresencePayload is the payload for presence frames.
type PresencePayload struct {
	Channel string `json:"channel"`
	UserID  string `json:"user_id"`
	Status  string `json:"status"` // "online", "offline", "away"
}

// TypingPayload is the payload for typing indicator frames.
type TypingPayload struct {
	Channel string `json:"channel"`
	UserID  string `json:"user_id"`
	Typing  bool   `json:"typing"`
}

// StreamPayload is the payload for stream frames.
type StreamPayload struct {
	StreamID string `json:"stream_id"`
	Action   string `json:"action"` // "start", "frame", "end"
	Sequence int64  `json:"sequence,omitempty"`
	Data     []byte `json:"data,omitempty"`
}

// NewFrame creates a new protocol frame.
func NewFrame(op OpCode, payload interface{}) (*Frame, error) {
	var p json.RawMessage
	if payload != nil {
		var err error
		p, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	return &Frame{
		Version:   Version,
		OpCode:    op,
		Timestamp: time.Now(),
		Payload:   p,
	}, nil
}

// NewErrorFrame creates a new error frame.
func NewErrorFrame(code int, message string) *Frame {
	return &Frame{
		Version:   Version,
		OpCode:    OpError,
		Timestamp: time.Now(),
		Error: &ProtocolError{
			Code:    code,
			Message: message,
		},
	}
}

// Encode encodes a frame to JSON bytes.
func (f *Frame) Encode() ([]byte, error) {
	return json.Marshal(f)
}

// Decode decodes JSON bytes into a frame.
func Decode(data []byte) (*Frame, error) {
	var f Frame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// DecodePayload decodes the frame payload into the given struct.
func (f *Frame) DecodePayload(v interface{}) error {
	if f.Payload == nil {
		return nil
	}
	return json.Unmarshal(f.Payload, v)
}

// IsError returns true if the frame is an error frame.
func (f *Frame) IsError() bool {
	return f.OpCode == OpError || f.Error != nil
}
