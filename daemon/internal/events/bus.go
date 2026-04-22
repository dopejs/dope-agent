package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Scope struct {
	SessionID            string `json:"sessionId,omitempty"`
	RunID                string `json:"runId,omitempty"`
	WorkflowID           string `json:"workflowId,omitempty"`
	WorkflowStepID       string `json:"workflowStepId,omitempty"`
	ScheduleID           string `json:"scheduleId,omitempty"`
	ScheduleAttemptID    string `json:"scheduleAttemptId,omitempty"`
	StepID               string `json:"stepId,omitempty"`
	ComputerUseSessionID string `json:"computerUseSessionId,omitempty"`
	ComputerUseActionID  string `json:"computerUseActionId,omitempty"`
	ConnectorID          string `json:"connectorId,omitempty"`
	CapabilityID         string `json:"capabilityId,omitempty"`
}

type Resource struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Event struct {
	EventID          string         `json:"eventId"`
	Sequence         int64          `json:"sequence"`
	EnvironmentScope string         `json:"-"`
	Category         string         `json:"category"`
	Name             string         `json:"name"`
	OccurredAt       time.Time      `json:"occurredAt"`
	Scope            Scope          `json:"scope"`
	Resource         Resource       `json:"resource"`
	Payload          map[string]any `json:"payload"`
}

type Filter struct {
	EnvironmentScope  string
	Category          string
	RunID             string
	SessionID         string
	ScheduleID        string
	ScheduleAttemptID string
	ResourceKind      string
	Cursor            int64
}

type Bus struct {
	mu          sync.RWMutex
	history     []Event
	subscribers map[int]subscriber
	nextID      int
	nextSeq     int64
	closed      bool
}

type subscriber struct {
	filter Filter
	ch     chan Event
}

func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[int]subscriber),
	}
}

func (b *Bus) Publish(event Event) Event {
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}

	b.mu.Lock()
	if event.Sequence == 0 {
		b.nextSeq++
		event.Sequence = b.nextSeq
	} else if event.Sequence > b.nextSeq {
		b.nextSeq = event.Sequence
	}
	b.history = append(b.history, event)

	subs := make([]subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	for _, sub := range subs {
		if !matches(sub.filter, event) {
			continue
		}
		select {
		case sub.ch <- event:
		default:
		}
	}

	return event
}

func (b *Bus) List(filter Filter) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	events := make([]Event, 0, len(b.history))
	for _, event := range b.history {
		if matches(filter, event) {
			events = append(events, event)
		}
	}

	return events
}

func (b *Bus) Subscribe(filter Filter) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}

	id := b.nextID
	b.nextID++

	ch := make(chan Event, 16)
	b.subscribers[id] = subscriber{
		filter: filter,
		ch:     ch,
	}

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		sub, ok := b.subscribers[id]
		if !ok {
			return
		}

		delete(b.subscribers, id)
		close(sub.ch)
	}

	return ch, unsubscribe
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for id, sub := range b.subscribers {
		delete(b.subscribers, id)
		close(sub.ch)
	}
}

func matches(filter Filter, event Event) bool {
	if filter.Cursor > 0 && event.Sequence <= filter.Cursor {
		return false
	}
	if filter.EnvironmentScope != "" && filter.EnvironmentScope != event.EnvironmentScope {
		return false
	}
	if filter.Category != "" && filter.Category != event.Category {
		return false
	}
	if filter.RunID != "" && filter.RunID != event.Scope.RunID {
		return false
	}
	if filter.SessionID != "" && filter.SessionID != event.Scope.SessionID {
		return false
	}
	if filter.ScheduleID != "" && filter.ScheduleID != event.Scope.ScheduleID {
		return false
	}
	if filter.ScheduleAttemptID != "" && filter.ScheduleAttemptID != event.Scope.ScheduleAttemptID {
		return false
	}
	if filter.ResourceKind != "" && filter.ResourceKind != event.Resource.Kind {
		return false
	}

	return true
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}

	return "evt_" + hex.EncodeToString(buf)
}

type contextKey string

const environmentScopeKey contextKey = "event_environment_scope"

func WithEnvironmentScope(ctx context.Context, scope string) context.Context {
	if scope == "" {
		return ctx
	}
	return context.WithValue(ctx, environmentScopeKey, scope)
}

func EnvironmentScopeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	scope, _ := ctx.Value(environmentScopeKey).(string)
	return scope
}
