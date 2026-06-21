package veclite

import (
	"fmt"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/veclite/internal/floats"
)

// MatchEvent represents an event when a new record matches a subscription.
type MatchEvent struct {
	// Record is the matching record.
	Record *Record
	// Score is the similarity score to the subscription query.
	Score float32
	// Timestamp is when the match was detected.
	Timestamp time.Time
	// SubscriptionID is the ID of the subscription that matched.
	SubscriptionID string
}

// Subscription represents an active subscription for matching records.
type Subscription struct {
	// ID is the unique identifier for this subscription.
	ID string
	// Query is the embedding vector to match against.
	Query []float32
	// Threshold is the minimum similarity score for a match.
	Threshold float32
	// Filters are additional filters to apply.
	Filters []Filter

	// channel is the internal event channel.
	channel chan MatchEvent
	// closed indicates if the subscription is closed.
	closed bool
	// mu protects the closed flag.
	mu sync.Mutex
}

// Events returns the channel for receiving match events.
func (s *Subscription) Events() <-chan MatchEvent {
	return s.channel
}

// Close closes the subscription and its event channel.
func (s *Subscription) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.channel)
	}
	return nil
}

// IsClosed returns true if the subscription is closed.
func (s *Subscription) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// send sends an event to the subscription channel if not closed.
func (s *Subscription) send(event MatchEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	select {
	case s.channel <- event:
		return true
	default:
		// Channel full, drop event
		return false
	}
}

// SubscriptionOption configures a subscription.
type SubscriptionOption interface {
	apply(*subscriptionConfig)
}

type subscriptionConfig struct {
	threshold  float32
	filters    []Filter
	bufferSize int
}

func defaultSubscriptionConfig() *subscriptionConfig {
	return &subscriptionConfig{
		threshold:  0.0,
		bufferSize: 100,
	}
}

type subscriptionOptionFunc func(*subscriptionConfig)

func (f subscriptionOptionFunc) apply(c *subscriptionConfig) {
	f(c)
}

// WithSubscriptionThreshold sets the minimum similarity threshold for matches.
func WithSubscriptionThreshold(threshold float32) SubscriptionOption {
	return subscriptionOptionFunc(func(c *subscriptionConfig) {
		c.threshold = threshold
	})
}

// WithSubscriptionFilter adds a filter to the subscription.
func WithSubscriptionFilter(f Filter) SubscriptionOption {
	return subscriptionOptionFunc(func(c *subscriptionConfig) {
		c.filters = append(c.filters, f)
	})
}

// WithSubscriptionBufferSize sets the event channel buffer size.
func WithSubscriptionBufferSize(size int) SubscriptionOption {
	return subscriptionOptionFunc(func(c *subscriptionConfig) {
		if size > 0 {
			c.bufferSize = size
		}
	})
}

// subscriptionManager manages subscriptions for a collection.
type subscriptionManager struct {
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
	distanceFunc  floats.DistanceFunc
	higherBetter  bool
}

func newSubscriptionManager(distanceType floats.DistanceType) *subscriptionManager {
	return &subscriptionManager{
		subscriptions: make(map[string]*Subscription),
		distanceFunc:  floats.GetDistanceFunc(distanceType),
		higherBetter:  floats.IsHigherBetter(distanceType),
	}
}

// subscribe creates a new subscription.
func (sm *subscriptionManager) subscribe(id string, query []float32, config *subscriptionConfig) *Subscription {
	sub := &Subscription{
		ID:        id,
		Query:     query,
		Threshold: config.threshold,
		Filters:   config.filters,
		channel:   make(chan MatchEvent, config.bufferSize),
	}

	sm.mu.Lock()
	sm.subscriptions[id] = sub
	sm.mu.Unlock()

	return sub
}

// unsubscribe removes a subscription.
func (sm *subscriptionManager) unsubscribe(id string) bool {
	sm.mu.Lock()
	sub, ok := sm.subscriptions[id]
	if ok {
		delete(sm.subscriptions, id)
	}
	sm.mu.Unlock()

	if ok {
		_ = sub.Close()
	}
	return ok
}

// notifyInsert checks all subscriptions for matches with a new record.
func (sm *subscriptionManager) notifyInsert(record *Record) {
	if record == nil || len(record.Vector) == 0 {
		return
	}

	sm.mu.RLock()
	subs := make([]*Subscription, 0, len(sm.subscriptions))
	for _, sub := range sm.subscriptions {
		subs = append(subs, sub)
	}
	sm.mu.RUnlock()

	now := time.Now()
	for _, sub := range subs {
		if sub.IsClosed() || len(sub.Query) != len(record.Vector) {
			continue
		}

		// Check filters
		matches := true
		for _, f := range sub.Filters {
			if !f.Match(record) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}

		// Calculate similarity
		score := sm.distanceFunc(sub.Query, record.Vector)

		// Check threshold
		if sm.higherBetter && score < sub.Threshold {
			continue
		}
		if !sm.higherBetter && score > sub.Threshold {
			continue
		}

		// Send event
		sub.send(MatchEvent{
			Record:         record.Clone(),
			Score:          score,
			Timestamp:      now,
			SubscriptionID: sub.ID,
		})
	}
}

// Collection subscription methods

// Subscribe creates a subscription that receives events when new records match the query.
// Returns the subscription which can be used to receive events and close the subscription.
func (c *Collection) Subscribe(query []float32, opts ...SubscriptionOption) (*Subscription, error) {
	if len(query) == 0 {
		return nil, ErrEmptyVector
	}

	if c.db == nil {
		return nil, ErrDatabaseClosed
	}

	c.mu.RLock()
	if c.dimension > 0 && len(query) != c.dimension {
		c.mu.RUnlock()
		return nil, &DimensionError{Expected: c.dimension, Got: len(query)}
	}
	c.mu.RUnlock()

	config := defaultSubscriptionConfig()
	for _, opt := range opts {
		opt.apply(config)
	}

	// Generate unique ID
	id := generateSubscriptionID()

	sm := c.db.getSubscriptionManager(c)
	sub := sm.subscribe(id, query, config)

	return sub, nil
}

// Unsubscribe removes a subscription by ID.
func (c *Collection) Unsubscribe(subscriptionID string) error {
	if c.db == nil {
		return ErrDatabaseClosed
	}

	sm := c.db.getSubscriptionManager(c)
	if !sm.unsubscribe(subscriptionID) {
		return &NotFoundError{Type: "subscription", ID: subscriptionID}
	}
	return nil
}

// notifySubscribers notifies all subscriptions about a new record.
// Called after successful insert operations.
func (c *Collection) notifySubscribers(record *Record) {
	if c.db == nil {
		return
	}
	sm := c.db.getSubscriptionManager(c)
	go sm.notifyInsert(record)
}

// subscriptionCounter is used to generate unique subscription IDs.
var subscriptionCounter uint64
var subscriptionCounterMu sync.Mutex

func generateSubscriptionID() string {
	subscriptionCounterMu.Lock()
	subscriptionCounter++
	id := subscriptionCounter
	subscriptionCounterMu.Unlock()

	return fmt.Sprintf("sub_%d_%d", time.Now().UnixNano(), id)
}
