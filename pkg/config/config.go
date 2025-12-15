// Package config provides configuration management for the realtime SDK.
package config

import (
	"errors"
	"time"
)

// Default configuration values
const (
	DefaultReconnectDelay    = 5 * time.Second
	DefaultMaxReconnects     = 10
	DefaultHeartbeatInterval = 30 * time.Second
	DefaultReadBufferSize    = 1024
	DefaultWriteBufferSize   = 1024
	DefaultWriteTimeout      = 10 * time.Second
	DefaultReadTimeout       = 60 * time.Second
)

// Config holds the SDK configuration parameters.
type Config struct {
	// APIKey is the authentication key for the API
	APIKey string

	// Endpoint is the WebSocket server URL
	Endpoint string

	// UserID is the user identifier (optional, for multi-user scenarios)
	UserID string

	// ReconnectDelay is the initial delay before attempting to reconnect
	ReconnectDelay time.Duration

	// MaxReconnects is the maximum number of reconnection attempts
	MaxReconnects int

	// HeartbeatInterval is the interval between heartbeat pings
	HeartbeatInterval time.Duration

	// ReadBufferSize is the WebSocket read buffer size in bytes
	ReadBufferSize int

	// WriteBufferSize is the WebSocket write buffer size in bytes
	WriteBufferSize int

	// WriteTimeout is the timeout for write operations
	WriteTimeout time.Duration

	// ReadTimeout is the timeout for read operations
	ReadTimeout time.Duration

	// EnableCompression enables WebSocket compression
	EnableCompression bool

	// Debug enables debug logging
	Debug bool
}

// Validation errors
var (
	ErrEmptyAPIKey   = errors.New("config: API key is required")
	ErrEmptyEndpoint = errors.New("config: endpoint is required")
	ErrInvalidDelay  = errors.New("config: reconnect delay must be positive")
)

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return ErrEmptyAPIKey
	}
	if c.Endpoint == "" {
		return ErrEmptyEndpoint
	}
	if c.ReconnectDelay < 0 {
		return ErrInvalidDelay
	}
	return nil
}

// WithDefaults returns a new Config with default values applied.
func (c Config) WithDefaults() Config {
	if c.ReconnectDelay == 0 {
		c.ReconnectDelay = DefaultReconnectDelay
	}
	if c.MaxReconnects == 0 {
		c.MaxReconnects = DefaultMaxReconnects
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = DefaultReadBufferSize
	}
	if c.WriteBufferSize == 0 {
		c.WriteBufferSize = DefaultWriteBufferSize
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = DefaultReadTimeout
	}
	return c
}

// Builder provides a fluent interface for building Config.
type Builder struct {
	config Config
}

// NewBuilder creates a new Config builder.
func NewBuilder() *Builder {
	return &Builder{
		config: Config{}.WithDefaults(),
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

// WithReadBufferSize sets the read buffer size.
func (b *Builder) WithReadBufferSize(size int) *Builder {
	b.config.ReadBufferSize = size
	return b
}

// WithWriteBufferSize sets the write buffer size.
func (b *Builder) WithWriteBufferSize(size int) *Builder {
	b.config.WriteBufferSize = size
	return b
}

// WithCompression enables or disables compression.
func (b *Builder) WithCompression(enabled bool) *Builder {
	b.config.EnableCompression = enabled
	return b
}

// WithDebug enables or disables debug mode.
func (b *Builder) WithDebug(enabled bool) *Builder {
	b.config.Debug = enabled
	return b
}

// Build creates the Config and validates it.
func (b *Builder) Build() (Config, error) {
	if err := b.config.Validate(); err != nil {
		return Config{}, err
	}
	return b.config, nil
}

// MustBuild creates the Config and panics on validation error.
func (b *Builder) MustBuild() Config {
	cfg, err := b.Build()
	if err != nil {
		panic(err)
	}
	return cfg
}
