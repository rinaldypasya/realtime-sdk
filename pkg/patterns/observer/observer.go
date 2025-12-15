// Package observer implements the Observer design pattern for event handling.
package observer

import (
	"sync"

	"github.com/rinaldypasya/realtime-sdk/pkg/events"
	"github.com/google/uuid"
)

// Observer interface defines the contract for event observers.
type Observer interface {
	OnEvent(event events.Event)
	ID() string
}

// Subject interface defines the contract for event subjects.
type Subject interface {
	Subscribe(eventType events.EventType, handler events.Handler) func()
	Unsubscribe(subscriptionID string)
	Notify(event events.Event)
}

// EventBus implements the Subject interface and manages event subscriptions.
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[events.EventType]map[string]*events.Subscription
	allHandlers   map[string]*events.Subscription // handlers for all events
}

// NewEventBus creates a new EventBus instance.
func NewEventBus() *EventBus {
	return &EventBus{
		subscriptions: make(map[events.EventType]map[string]*events.Subscription),
		allHandlers:   make(map[string]*events.Subscription),
	}
}

// Subscribe registers a handler for a specific event type.
// Returns an unsubscribe function.
func (eb *EventBus) Subscribe(eventType events.EventType, handler events.Handler) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subID := uuid.New().String()
	subscription := &events.Subscription{
		ID:        subID,
		EventType: eventType,
		Handler:   handler,
	}

	if eb.subscriptions[eventType] == nil {
		eb.subscriptions[eventType] = make(map[string]*events.Subscription)
	}
	eb.subscriptions[eventType][subID] = subscription

	// Return unsubscribe function
	return func() {
		eb.Unsubscribe(subID)
	}
}

// SubscribeAll registers a handler for all event types.
func (eb *EventBus) SubscribeAll(handler events.Handler) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subID := uuid.New().String()
	subscription := &events.Subscription{
		ID:      subID,
		Handler: handler,
	}

	eb.allHandlers[subID] = subscription

	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		delete(eb.allHandlers, subID)
	}
}

// Unsubscribe removes a subscription by ID.
func (eb *EventBus) Unsubscribe(subscriptionID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for eventType, subs := range eb.subscriptions {
		if _, exists := subs[subscriptionID]; exists {
			delete(eb.subscriptions[eventType], subscriptionID)
			return
		}
	}

	delete(eb.allHandlers, subscriptionID)
}

// Notify sends an event to all relevant subscribers.
func (eb *EventBus) Notify(event events.Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Notify specific event type subscribers
	if subs, exists := eb.subscriptions[event.Type]; exists {
		for _, sub := range subs {
			go sub.Handler(event)
		}
	}

	// Notify all-event handlers
	for _, sub := range eb.allHandlers {
		go sub.Handler(event)
	}
}

// NotifySync sends an event synchronously (blocking).
func (eb *EventBus) NotifySync(event events.Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Notify specific event type subscribers
	if subs, exists := eb.subscriptions[event.Type]; exists {
		for _, sub := range subs {
			sub.Handler(event)
		}
	}

	// Notify all-event handlers
	for _, sub := range eb.allHandlers {
		sub.Handler(event)
	}
}

// HasSubscribers returns true if there are subscribers for the event type.
func (eb *EventBus) HasSubscribers(eventType events.EventType) bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if subs, exists := eb.subscriptions[eventType]; exists {
		return len(subs) > 0
	}
	return len(eb.allHandlers) > 0
}

// SubscriberCount returns the number of subscribers for an event type.
func (eb *EventBus) SubscriberCount(eventType events.EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	count := 0
	if subs, exists := eb.subscriptions[eventType]; exists {
		count = len(subs)
	}
	return count + len(eb.allHandlers)
}

// Clear removes all subscriptions.
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscriptions = make(map[events.EventType]map[string]*events.Subscription)
	eb.allHandlers = make(map[string]*events.Subscription)
}

// BaseObserver provides a basic implementation of the Observer interface.
type BaseObserver struct {
	id      string
	handler events.Handler
}

// NewBaseObserver creates a new BaseObserver.
func NewBaseObserver(handler events.Handler) *BaseObserver {
	return &BaseObserver{
		id:      uuid.New().String(),
		handler: handler,
	}
}

// OnEvent handles incoming events.
func (o *BaseObserver) OnEvent(event events.Event) {
	if o.handler != nil {
		o.handler(event)
	}
}

// ID returns the observer's unique identifier.
func (o *BaseObserver) ID() string {
	return o.id
}
