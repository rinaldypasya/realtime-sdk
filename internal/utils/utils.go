// Package utils provides utility functions for the realtime SDK.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Snowflake generates unique IDs similar to Twitter's Snowflake.
type Snowflake struct {
	mu        sync.Mutex
	epoch     int64
	nodeID    int64
	sequence  int64
	lastTime  int64
}

const (
	snowflakeEpoch     = 1609459200000 // 2021-01-01 00:00:00 UTC in milliseconds
	nodeBits           = 10
	sequenceBits       = 12
	maxNodeID          = -1 ^ (-1 << nodeBits)
	maxSequence        = -1 ^ (-1 << sequenceBits)
	nodeShift          = sequenceBits
	timestampShift     = nodeBits + sequenceBits
)

// NewSnowflake creates a new Snowflake ID generator.
func NewSnowflake(nodeID int64) *Snowflake {
	if nodeID < 0 || nodeID > maxNodeID {
		nodeID = nodeID & maxNodeID
	}
	return &Snowflake{
		epoch:  snowflakeEpoch,
		nodeID: nodeID,
	}
}

// Generate generates a new unique ID.
func (s *Snowflake) Generate() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == s.lastTime {
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// Wait for next millisecond
			for now <= s.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}

	s.lastTime = now

	return ((now - s.epoch) << timestampShift) |
		(s.nodeID << nodeShift) |
		s.sequence
}

// RandomString generates a random hex string of the given length.
func RandomString(length int) string {
	bytes := make([]byte, (length+1)/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		InitialWait: time.Second,
		MaxWait:     time.Minute,
		Multiplier:  2.0,
	}
}

// Retry executes a function with exponential backoff retry.
func Retry(fn func() error, config RetryConfig) error {
	var lastErr error
	wait := config.InitialWait

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			time.Sleep(wait)

			wait = time.Duration(float64(wait) * config.Multiplier)
			if wait > config.MaxWait {
				wait = config.MaxWait
			}
			continue
		}
		return nil
	}

	return lastErr
}

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Wait waits until a request is allowed.
func (r *RateLimiter) Wait() {
	for !r.Allow() {
		time.Sleep(time.Millisecond * 10)
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu               sync.Mutex
	failureThreshold int
	resetTimeout     time.Duration
	failures         int
	lastFailure      time.Time
	state            CircuitState
}

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: threshold,
		resetTimeout:     resetTimeout,
		state:            CircuitClosed,
	}
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return true
	}
}

// Success records a successful operation.
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = CircuitClosed
}

// Failure records a failed operation.
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
