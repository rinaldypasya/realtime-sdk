// Package handlers provides message and event handlers for the realtime SDK.
package handlers

import (
	"context"
	"sync"

	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/rinaldypasya/realtime-sdk/pkg/message"
)

// MessageHandler is a function that handles incoming messages.
type MessageHandler func(ctx context.Context, msg *message.Message) error

// EventHandler is a function that handles events.
type EventHandler func(ctx context.Context, event events.Event) error

// Middleware wraps a handler with additional functionality.
type Middleware func(MessageHandler) MessageHandler

// HandlerRegistry manages message and event handlers.
type HandlerRegistry struct {
	mu              sync.RWMutex
	messageHandlers map[string][]MessageHandler
	eventHandlers   map[events.EventType][]EventHandler
	middlewares     []Middleware
	defaultHandler  MessageHandler
}

// NewHandlerRegistry creates a new handler registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		messageHandlers: make(map[string][]MessageHandler),
		eventHandlers:   make(map[events.EventType][]EventHandler),
		middlewares:     make([]Middleware, 0),
	}
}

// OnMessage registers a handler for messages on a specific channel.
func (r *HandlerRegistry) OnMessage(channel string, handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.messageHandlers[channel] = append(r.messageHandlers[channel], handler)
}

// OnAllMessages registers a handler for all messages.
func (r *HandlerRegistry) OnAllMessages(handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.messageHandlers["*"] = append(r.messageHandlers["*"], handler)
}

// SetDefaultHandler sets the default message handler.
func (r *HandlerRegistry) SetDefaultHandler(handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultHandler = handler
}

// OnEvent registers a handler for a specific event type.
func (r *HandlerRegistry) OnEvent(eventType events.EventType, handler EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.eventHandlers[eventType] = append(r.eventHandlers[eventType], handler)
}

// Use adds middleware to the handler chain.
func (r *HandlerRegistry) Use(middleware Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.middlewares = append(r.middlewares, middleware)
}

// HandleMessage dispatches a message to registered handlers.
func (r *HandlerRegistry) HandleMessage(ctx context.Context, msg *message.Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Apply middlewares
	handler := r.wrapWithMiddleware(r.dispatchMessage)

	return handler(ctx, msg)
}

// dispatchMessage dispatches the message to appropriate handlers.
func (r *HandlerRegistry) dispatchMessage(ctx context.Context, msg *message.Message) error {
	// Channel-specific handlers
	if handlers, ok := r.messageHandlers[msg.Channel]; ok {
		for _, h := range handlers {
			if err := h(ctx, msg); err != nil {
				return err
			}
		}
	}

	// Wildcard handlers
	if handlers, ok := r.messageHandlers["*"]; ok {
		for _, h := range handlers {
			if err := h(ctx, msg); err != nil {
				return err
			}
		}
	}

	// Default handler
	if r.defaultHandler != nil && len(r.messageHandlers[msg.Channel]) == 0 {
		return r.defaultHandler(ctx, msg)
	}

	return nil
}

// wrapWithMiddleware applies all middlewares to a handler.
func (r *HandlerRegistry) wrapWithMiddleware(handler MessageHandler) MessageHandler {
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	return handler
}

// HandleEvent dispatches an event to registered handlers.
func (r *HandlerRegistry) HandleEvent(ctx context.Context, event events.Event) error {
	r.mu.RLock()
	handlers := r.eventHandlers[event.Type]
	r.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// Clear removes all registered handlers.
func (r *HandlerRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.messageHandlers = make(map[string][]MessageHandler)
	r.eventHandlers = make(map[events.EventType][]EventHandler)
	r.middlewares = make([]Middleware, 0)
	r.defaultHandler = nil
}

// LoggingMiddleware creates a middleware that logs messages.
func LoggingMiddleware(logger func(string, ...interface{})) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *message.Message) error {
			logger("Handling message: id=%s, channel=%s, type=%s",
				msg.ID, msg.Channel, msg.Type)
			err := next(ctx, msg)
			if err != nil {
				logger("Error handling message: %v", err)
			}
			return err
		}
	}
}

// RecoveryMiddleware creates a middleware that recovers from panics.
func RecoveryMiddleware(onPanic func(interface{})) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *message.Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if onPanic != nil {
						onPanic(r)
					}
					// Convert panic to error
					if e, ok := r.(error); ok {
						err = e
					}
				}
			}()
			return next(ctx, msg)
		}
	}
}

// TimeoutMiddleware creates a middleware that enforces a timeout.
func TimeoutMiddleware(ctx context.Context) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(_ context.Context, msg *message.Message) error {
			done := make(chan error, 1)
			go func() {
				done <- next(ctx, msg)
			}()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-done:
				return err
			}
		}
	}
}

// FilterMiddleware creates a middleware that filters messages.
func FilterMiddleware(predicate func(*message.Message) bool) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *message.Message) error {
			if !predicate(msg) {
				return nil // Skip message
			}
			return next(ctx, msg)
		}
	}
}
