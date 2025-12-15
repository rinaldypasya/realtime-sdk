// Package connection provides connection management for the realtime SDK.
package connection

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/rinaldypasya/realtime-sdk/pkg/config"
	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/observer"
	"github.com/rinaldypasya/realtime-sdk/pkg/patterns/strategy"
)

// Common errors
var (
	ErrNotConnected     = errors.New("connection: not connected")
	ErrAlreadyConnected = errors.New("connection: already connected")
	ErrMaxReconnects    = errors.New("connection: max reconnection attempts reached")
	ErrClosed           = errors.New("connection: manager is closed")
)

// State represents the connection state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

// String returns the string representation of the state.
func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Manager handles connection lifecycle and reconnection logic.
type Manager struct {
	mu              sync.RWMutex
	config          config.Config
	strategy        strategy.ConnectionStrategy
	eventBus        *observer.EventBus
	state           State
	reconnectCount  int
	lastConnectTime time.Time
	closeChan       chan struct{}
	doneChan        chan struct{}
	wg              sync.WaitGroup
}

// NewManager creates a new connection manager.
func NewManager(cfg config.Config, connStrategy strategy.ConnectionStrategy, eventBus *observer.EventBus) *Manager {
	return &Manager{
		config:    cfg,
		strategy:  connStrategy,
		eventBus:  eventBus,
		state:     StateDisconnected,
		closeChan: make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

// Connect establishes a connection.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.state == StateConnected {
		m.mu.Unlock()
		return ErrAlreadyConnected
	}
	if m.state == StateClosed {
		m.mu.Unlock()
		return ErrClosed
	}
	m.state = StateConnecting
	m.mu.Unlock()

	m.eventBus.Notify(events.NewEvent(events.Reconnecting, nil))

	err := m.strategy.Connect(ctx)
	if err != nil {
		m.mu.Lock()
		m.state = StateDisconnected
		m.mu.Unlock()
		m.eventBus.Notify(events.NewEventWithError(events.ConnectionError, err))
		return err
	}

	m.mu.Lock()
	m.state = StateConnected
	m.lastConnectTime = time.Now()
	m.reconnectCount = 0
	m.mu.Unlock()

	m.eventBus.Notify(events.NewEvent(events.Connected, &events.ConnectionEvent{
		Endpoint: m.config.Endpoint,
	}))

	// Start event forwarding
	m.wg.Add(2)
	go m.forwardMessages()
	go m.forwardEvents()

	return nil
}

// forwardMessages forwards messages from the strategy to the event bus.
func (m *Manager) forwardMessages() {
	defer m.wg.Done()

	for {
		select {
		case <-m.closeChan:
			return
		case msg, ok := <-m.strategy.Receive():
			if !ok {
				return
			}
			m.eventBus.Notify(events.NewEvent(events.MessageReceived, msg))
		}
	}
}

// forwardEvents forwards events from the strategy to the event bus.
func (m *Manager) forwardEvents() {
	defer m.wg.Done()

	for {
		select {
		case <-m.closeChan:
			return
		case evt, ok := <-m.strategy.Events():
			if !ok {
				return
			}
			m.eventBus.Notify(evt)

			// Handle connection loss
			if evt.Type == events.ConnectionLost || evt.Type == events.Disconnected {
				m.handleConnectionLost()
			}
		}
	}
}

// handleConnectionLost handles connection loss and triggers reconnection.
func (m *Manager) handleConnectionLost() {
	m.mu.Lock()
	if m.state == StateClosed || m.state == StateReconnecting {
		m.mu.Unlock()
		return
	}
	m.state = StateReconnecting
	m.mu.Unlock()

	go m.reconnect()
}

// reconnect attempts to reconnect with exponential backoff.
func (m *Manager) reconnect() {
	for {
		m.mu.Lock()
		if m.state == StateClosed {
			m.mu.Unlock()
			return
		}
		if m.reconnectCount >= m.config.MaxReconnects {
			m.state = StateDisconnected
			m.mu.Unlock()
			m.eventBus.Notify(events.NewEventWithError(events.ConnectionError, ErrMaxReconnects))
			return
		}
		m.reconnectCount++
		count := m.reconnectCount
		m.mu.Unlock()

		// Calculate backoff delay with exponential increase
		delay := m.calculateBackoff(count)
		m.eventBus.Notify(events.NewEvent(events.Reconnecting, &events.ConnectionEvent{
			ReconnectCount: count,
		}))

		select {
		case <-m.closeChan:
			return
		case <-time.After(delay):
		}

		// Attempt to connect
		ctx, cancel := context.WithTimeout(context.Background(), m.config.WriteTimeout)
		err := m.strategy.Connect(ctx)
		cancel()

		if err == nil {
			m.mu.Lock()
			m.state = StateConnected
			m.lastConnectTime = time.Now()
			m.mu.Unlock()

			m.eventBus.Notify(events.NewEvent(events.Reconnected, &events.ConnectionEvent{
				Endpoint:       m.config.Endpoint,
				ReconnectCount: count,
			}))
			return
		}

		m.eventBus.Notify(events.NewEventWithError(events.ConnectionError, err))
	}
}

// calculateBackoff calculates the backoff delay using exponential backoff.
func (m *Manager) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: delay * 2^attempt, capped at 60 seconds
	backoff := float64(m.config.ReconnectDelay) * math.Pow(2, float64(attempt-1))
	maxBackoff := float64(60 * time.Second)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return time.Duration(backoff)
}

// Disconnect closes the connection.
func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	if m.state == StateDisconnected || m.state == StateClosed {
		m.mu.Unlock()
		return nil
	}
	m.state = StateDisconnected
	m.mu.Unlock()

	err := m.strategy.Disconnect(ctx)
	if err != nil {
		return err
	}

	m.eventBus.Notify(events.NewEvent(events.Disconnected, nil))
	return nil
}

// Close permanently closes the connection manager.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	if m.state == StateClosed {
		m.mu.Unlock()
		return nil
	}
	m.state = StateClosed
	close(m.closeChan)
	m.mu.Unlock()

	// Disconnect the strategy
	if err := m.strategy.Disconnect(ctx); err != nil {
		return err
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	close(m.doneChan)
	return nil
}

// Send sends a message through the connection.
func (m *Manager) Send(ctx context.Context, msg *message.Message) error {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	if state != StateConnected {
		return ErrNotConnected
	}

	return m.strategy.Send(ctx, msg)
}

// SendJoinChannel sends a join channel request.
func (m *Manager) SendJoinChannel(ctx context.Context, channelID string) error {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	if state != StateConnected {
		return ErrNotConnected
	}

	return m.strategy.SendJoinChannel(ctx, channelID)
}

// SendLeaveChannel sends a leave channel request.
func (m *Manager) SendLeaveChannel(ctx context.Context, channelID string) error {
	m.mu.RLock()
	state := m.state
	m.mu.RUnlock()

	if state != StateConnected {
		return ErrNotConnected
	}

	return m.strategy.SendLeaveChannel(ctx, channelID)
}

// State returns the current connection state.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// IsConnected returns true if connected.
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateConnected
}

// ReconnectCount returns the current reconnection attempt count.
func (m *Manager) ReconnectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reconnectCount
}

// LastConnectTime returns when the connection was last established.
func (m *Manager) LastConnectTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastConnectTime
}

// Done returns a channel that's closed when the manager is closed.
func (m *Manager) Done() <-chan struct{} {
	return m.doneChan
}
