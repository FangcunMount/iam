package event

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	SourceAPIServer = "iam-apiserver"
)

// DomainEvent describes a business fact that can be routed through MQ or
// staged into the transactional outbox.
type DomainEvent interface {
	EventID() string
	EventType() string
	OccurredAt() time.Time
	AggregateType() string
	AggregateID() string
	Payload() any
}

// Stager stages durable events inside the caller's active transaction.
type Stager interface {
	Stage(ctx context.Context, events ...DomainEvent) error
}

// Publisher publishes events to their configured transport.
type Publisher interface {
	Publish(ctx context.Context, event DomainEvent) error
	PublishAll(ctx context.Context, events []DomainEvent) error
}

// BaseEvent provides the stable event identity and routing fields.
type BaseEvent struct {
	ID                 string
	EventTypeValue     string
	OccurredAtValue    time.Time
	AggregateTypeValue string
	AggregateIDValue   string
}

func NewBaseEvent(eventType, aggregateType, aggregateID string) BaseEvent {
	return BaseEvent{
		ID:                 uuid.New().String(),
		EventTypeValue:     eventType,
		OccurredAtValue:    time.Now(),
		AggregateTypeValue: aggregateType,
		AggregateIDValue:   aggregateID,
	}
}

func (e BaseEvent) EventID() string       { return e.ID }
func (e BaseEvent) EventType() string     { return e.EventTypeValue }
func (e BaseEvent) OccurredAt() time.Time { return e.OccurredAtValue }
func (e BaseEvent) AggregateType() string { return e.AggregateTypeValue }
func (e BaseEvent) AggregateID() string   { return e.AggregateIDValue }

// Event is a generic domain event with a JSON-serializable payload.
type Event[T any] struct {
	BaseEvent
	Data T
}

func New[T any](eventType, aggregateType, aggregateID string, data T) Event[T] {
	return Event[T]{
		BaseEvent: NewBaseEvent(eventType, aggregateType, aggregateID),
		Data:      data,
	}
}

func (e Event[T]) Payload() any { return e.Data }
