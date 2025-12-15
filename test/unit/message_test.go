package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageJSON(t *testing.T) {
	msg := &message.Message{
		ID:        "test-id",
		Type:      message.TypeText,
		Channel:   "general",
		Content:   "Hello, world!",
		SenderID:  "user-123",
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		Status:    message.StatusSent,
		Priority:  message.NormalPriority,
	}

	// Serialize
	data, err := msg.ToJSON()
	require.NoError(t, err)

	// Deserialize
	decoded, err := message.FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.Channel, decoded.Channel)
	assert.Equal(t, msg.Content, decoded.Content)
	assert.Equal(t, msg.SenderID, decoded.SenderID)
	assert.Equal(t, msg.Status, decoded.Status)
}

func TestMessageIsEdited(t *testing.T) {
	msg := &message.Message{}
	assert.False(t, msg.IsEdited())

	now := time.Now()
	msg.EditedAt = &now
	assert.True(t, msg.IsEdited())
}

func TestMessageIsDeleted(t *testing.T) {
	msg := &message.Message{}
	assert.False(t, msg.IsDeleted())

	now := time.Now()
	msg.DeletedAt = &now
	assert.True(t, msg.IsDeleted())
}

func TestMessageIsExpired(t *testing.T) {
	// No TTL = never expires
	msg := &message.Message{
		Timestamp: time.Now().Add(-time.Hour),
	}
	assert.False(t, msg.IsExpired())

	// Not expired yet
	msg.TTL = time.Hour * 2
	assert.False(t, msg.IsExpired())

	// Expired
	msg.TTL = time.Minute
	assert.True(t, msg.IsExpired())
}

func TestMessageHasAttachments(t *testing.T) {
	msg := &message.Message{}
	assert.False(t, msg.HasAttachments())

	msg.Attachments = []message.Attachment{
		{ID: "att-1", Name: "file.txt"},
	}
	assert.True(t, msg.HasAttachments())
}

func TestMessageClone(t *testing.T) {
	msg := &message.Message{
		ID:       "original",
		Channel:  "general",
		Content:  "Hello",
		Metadata: map[string]interface{}{"key": "value"},
		Attachments: []message.Attachment{
			{ID: "att-1"},
		},
		Mentions: []string{"user-1"},
	}

	clone := msg.Clone()

	// Verify clone has same values
	assert.Equal(t, msg.ID, clone.ID)
	assert.Equal(t, msg.Channel, clone.Channel)
	assert.Equal(t, msg.Content, clone.Content)

	// Verify clone has independent copies
	clone.Metadata["key"] = "modified"
	assert.Equal(t, "value", msg.Metadata["key"])

	clone.Attachments[0].ID = "modified"
	assert.Equal(t, "att-1", msg.Attachments[0].ID)
}

func TestMessageBuilder(t *testing.T) {
	msg := message.NewBuilder().
		WithID("test-id").
		WithType(message.TypeImage).
		WithChannel("photos").
		WithContent("https://example.com/image.jpg").
		WithSenderID("user-123").
		WithPriority(message.HighPriority).
		WithReplyTo("reply-to-id").
		WithMetadata("custom", "data").
		WithMention("user-456").
		WithTTL(time.Hour).
		Build()

	assert.Equal(t, "test-id", msg.ID)
	assert.Equal(t, message.TypeImage, msg.Type)
	assert.Equal(t, "photos", msg.Channel)
	assert.Equal(t, "https://example.com/image.jpg", msg.Content)
	assert.Equal(t, "user-123", msg.SenderID)
	assert.Equal(t, message.HighPriority, msg.Priority)
	assert.Equal(t, "reply-to-id", msg.ReplyTo)
	assert.Equal(t, "data", msg.Metadata["custom"])
	assert.Contains(t, msg.Mentions, "user-456")
	assert.Equal(t, time.Hour, msg.TTL)
}

func TestMessageBuilderWithAttachment(t *testing.T) {
	attachment := message.Attachment{
		ID:       "att-1",
		Type:     "image",
		Name:     "photo.jpg",
		URL:      "https://example.com/photo.jpg",
		Size:     1024,
		MimeType: "image/jpeg",
	}

	msg := message.NewBuilder().
		WithChannel("general").
		WithContent("Check this out!").
		WithAttachment(attachment).
		Build()

	assert.Len(t, msg.Attachments, 1)
	assert.Equal(t, "photo.jpg", msg.Attachments[0].Name)
}

func TestEnvelope(t *testing.T) {
	msg := &message.Message{
		ID:      "msg-1",
		Channel: "general",
		Content: "Hello",
	}

	envelope := message.Envelope{
		Action:    message.ActionSend,
		Message:   msg,
		Channel:   "general",
		Timestamp: time.Now(),
		RequestID: "req-123",
	}

	// Serialize
	data, err := json.Marshal(envelope)
	require.NoError(t, err)

	// Deserialize
	var decoded message.Envelope
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, message.ActionSend, decoded.Action)
	assert.Equal(t, "msg-1", decoded.Message.ID)
	assert.Equal(t, "req-123", decoded.RequestID)
}
