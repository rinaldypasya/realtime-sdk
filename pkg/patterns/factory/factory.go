// Package factory implements the Factory design pattern for creating objects.
package factory

import (
	"errors"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/google/uuid"
)

// MessageFactory creates message objects.
type MessageFactory struct {
	defaultSenderID string
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(senderID string) *MessageFactory {
	return &MessageFactory{
		defaultSenderID: senderID,
	}
}

// CreateTextMessage creates a new text message.
func (f *MessageFactory) CreateTextMessage(channel, content string) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeText,
		Channel:   channel,
		Content:   content,
		SenderID:  f.defaultSenderID,
		Timestamp: time.Now(),
		Status:    message.StatusPending,
	}
}

// CreateImageMessage creates a new image message.
func (f *MessageFactory) CreateImageMessage(channel, imageURL string) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeImage,
		Channel:   channel,
		Content:   imageURL,
		SenderID:  f.defaultSenderID,
		Timestamp: time.Now(),
		Status:    message.StatusPending,
		Metadata: map[string]interface{}{
			"type": "image",
			"url":  imageURL,
		},
	}
}

// CreateVideoMessage creates a new video message.
func (f *MessageFactory) CreateVideoMessage(channel, videoURL string) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeVideo,
		Channel:   channel,
		Content:   videoURL,
		SenderID:  f.defaultSenderID,
		Timestamp: time.Now(),
		Status:    message.StatusPending,
		Metadata: map[string]interface{}{
			"type": "video",
			"url":  videoURL,
		},
	}
}

// CreateFileMessage creates a new file message.
func (f *MessageFactory) CreateFileMessage(channel, fileName, fileURL string, fileSize int64) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeFile,
		Channel:   channel,
		Content:   fileName,
		SenderID:  f.defaultSenderID,
		Timestamp: time.Now(),
		Status:    message.StatusPending,
		Metadata: map[string]interface{}{
			"type":     "file",
			"name":     fileName,
			"url":      fileURL,
			"size":     fileSize,
		},
	}
}

// CreateSystemMessage creates a system notification message.
func (f *MessageFactory) CreateSystemMessage(channel, content string) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeSystem,
		Channel:   channel,
		Content:   content,
		SenderID:  "system",
		Timestamp: time.Now(),
		Status:    message.StatusDelivered,
	}
}

// CreateReplyMessage creates a reply to another message.
func (f *MessageFactory) CreateReplyMessage(channel, content, replyToID string) *message.Message {
	return &message.Message{
		ID:        uuid.New().String(),
		Type:      message.TypeText,
		Channel:   channel,
		Content:   content,
		SenderID:  f.defaultSenderID,
		Timestamp: time.Now(),
		Status:    message.StatusPending,
		ReplyTo:   replyToID,
	}
}

// EventFactory creates event objects.
type EventFactory struct{}

// NewEventFactory creates a new event factory.
func NewEventFactory() *EventFactory {
	return &EventFactory{}
}

// CreateConnectionEvent creates a connection-related event.
func (f *EventFactory) CreateConnectionEvent(eventType events.EventType, endpoint string, reconnectCount int) events.Event {
	return events.NewEvent(eventType, &events.ConnectionEvent{
		Endpoint:       endpoint,
		ReconnectCount: reconnectCount,
	})
}

// CreateMessageEvent creates a message-related event.
func (f *EventFactory) CreateMessageEvent(eventType events.EventType, msg *message.Message) events.Event {
	return events.NewEvent(eventType, &events.MessageEvent{
		MessageID: msg.ID,
		Channel:   msg.Channel,
		Content:   msg.Content,
		SenderID:  msg.SenderID,
		Timestamp: msg.Timestamp,
	})
}

// CreatePresenceEvent creates a presence-related event.
func (f *EventFactory) CreatePresenceEvent(eventType events.EventType, userID, username, channel, status string) events.Event {
	return events.NewEvent(eventType, &events.PresenceEvent{
		UserID:    userID,
		Username:  username,
		Channel:   channel,
		Status:    status,
		Timestamp: time.Now(),
	})
}

// CreateErrorEvent creates an error event.
func (f *EventFactory) CreateErrorEvent(eventType events.EventType, code int, message string, details map[string]interface{}) events.Event {
	return events.NewEvent(eventType, &events.ErrorEvent{
		Code:    code,
		Message: message,
		Details: details,
	})
}

// StreamFactory creates stream-related objects.
type StreamFactory struct {
	defaultBitrate int
}

// NewStreamFactory creates a new stream factory.
func NewStreamFactory(defaultBitrate int) *StreamFactory {
	return &StreamFactory{
		defaultBitrate: defaultBitrate,
	}
}

// Stream represents a video/audio stream.
type Stream struct {
	ID        string
	Type      string
	Bitrate   int
	StartTime time.Time
	EndTime   time.Time
	Active    bool
	Metadata  map[string]interface{}
}

// CreateVideoStream creates a new video stream.
func (f *StreamFactory) CreateVideoStream(streamType string) *Stream {
	return &Stream{
		ID:        uuid.New().String(),
		Type:      streamType,
		Bitrate:   f.defaultBitrate,
		StartTime: time.Now(),
		Active:    true,
		Metadata:  make(map[string]interface{}),
	}
}

// Frame represents a single frame in a stream.
type Frame struct {
	ID        string
	StreamID  string
	Sequence  int64
	Data      []byte
	Timestamp time.Time
}

// CreateFrame creates a new frame for a stream.
func (f *StreamFactory) CreateFrame(streamID string, sequence int64, data []byte) *Frame {
	return &Frame{
		ID:        uuid.New().String(),
		StreamID:  streamID,
		Sequence:  sequence,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// AbstractFactory provides a unified interface for creating related objects.
type AbstractFactory interface {
	CreateMessage(messageType message.Type, channel, content string) (*message.Message, error)
	CreateEvent(eventType events.EventType, payload interface{}) (events.Event, error)
}

// RealtimeFactory is a concrete implementation of AbstractFactory.
type RealtimeFactory struct {
	messageFactory *MessageFactory
	eventFactory   *EventFactory
}

// NewRealtimeFactory creates a new realtime factory.
func NewRealtimeFactory(senderID string) *RealtimeFactory {
	return &RealtimeFactory{
		messageFactory: NewMessageFactory(senderID),
		eventFactory:   NewEventFactory(),
	}
}

// CreateMessage creates a message based on type.
func (f *RealtimeFactory) CreateMessage(msgType message.Type, channel, content string) (*message.Message, error) {
	switch msgType {
	case message.TypeText:
		return f.messageFactory.CreateTextMessage(channel, content), nil
	case message.TypeImage:
		return f.messageFactory.CreateImageMessage(channel, content), nil
	case message.TypeVideo:
		return f.messageFactory.CreateVideoMessage(channel, content), nil
	case message.TypeSystem:
		return f.messageFactory.CreateSystemMessage(channel, content), nil
	default:
		return nil, errors.New("unknown message type")
	}
}

// CreateEvent creates an event.
func (f *RealtimeFactory) CreateEvent(eventType events.EventType, payload interface{}) (events.Event, error) {
	return events.NewEvent(eventType, payload), nil
}

// MessageFactory accessor
func (f *RealtimeFactory) Messages() *MessageFactory {
	return f.messageFactory
}

// EventFactory accessor
func (f *RealtimeFactory) Events() *EventFactory {
	return f.eventFactory
}
