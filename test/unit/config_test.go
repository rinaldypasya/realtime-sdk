package unit

import (
	"testing"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr error
	}{
		{
			name: "valid config",
			config: config.Config{
				APIKey:   "test-key",
				Endpoint: "wss://example.com/ws",
			},
			wantErr: nil,
		},
		{
			name: "missing api key",
			config: config.Config{
				Endpoint: "wss://example.com/ws",
			},
			wantErr: config.ErrEmptyAPIKey,
		},
		{
			name: "missing endpoint",
			config: config.Config{
				APIKey: "test-key",
			},
			wantErr: config.ErrEmptyEndpoint,
		},
		{
			name: "negative reconnect delay",
			config: config.Config{
				APIKey:         "test-key",
				Endpoint:       "wss://example.com/ws",
				ReconnectDelay: -time.Second,
			},
			wantErr: config.ErrInvalidDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestConfigWithDefaults(t *testing.T) {
	cfg := config.Config{
		APIKey:   "test-key",
		Endpoint: "wss://example.com/ws",
	}

	cfg = cfg.WithDefaults()

	assert.Equal(t, config.DefaultReconnectDelay, cfg.ReconnectDelay)
	assert.Equal(t, config.DefaultMaxReconnects, cfg.MaxReconnects)
	assert.Equal(t, config.DefaultHeartbeatInterval, cfg.HeartbeatInterval)
	assert.Equal(t, config.DefaultReadBufferSize, cfg.ReadBufferSize)
	assert.Equal(t, config.DefaultWriteBufferSize, cfg.WriteBufferSize)
}

func TestConfigBuilder(t *testing.T) {
	cfg, err := config.NewBuilder().
		WithAPIKey("my-api-key").
		WithEndpoint("wss://example.com/realtime").
		WithReconnectDelay(time.Second * 10).
		WithMaxReconnects(5).
		WithHeartbeatInterval(time.Second * 60).
		WithReadBufferSize(2048).
		WithWriteBufferSize(2048).
		WithCompression(true).
		WithDebug(true).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, "my-api-key", cfg.APIKey)
	assert.Equal(t, "wss://example.com/realtime", cfg.Endpoint)
	assert.Equal(t, time.Second*10, cfg.ReconnectDelay)
	assert.Equal(t, 5, cfg.MaxReconnects)
	assert.Equal(t, time.Second*60, cfg.HeartbeatInterval)
	assert.Equal(t, 2048, cfg.ReadBufferSize)
	assert.Equal(t, 2048, cfg.WriteBufferSize)
	assert.True(t, cfg.EnableCompression)
	assert.True(t, cfg.Debug)
}

func TestConfigBuilderValidation(t *testing.T) {
	// Missing required fields
	_, err := config.NewBuilder().Build()
	assert.Error(t, err)

	// Valid build
	_, err = config.NewBuilder().
		WithAPIKey("key").
		WithEndpoint("wss://example.com").
		Build()
	assert.NoError(t, err)
}

func TestConfigMustBuild(t *testing.T) {
	// Should panic with invalid config
	assert.Panics(t, func() {
		config.NewBuilder().MustBuild()
	})

	// Should not panic with valid config
	assert.NotPanics(t, func() {
		config.NewBuilder().
			WithAPIKey("key").
			WithEndpoint("wss://example.com").
			MustBuild()
	})
}
