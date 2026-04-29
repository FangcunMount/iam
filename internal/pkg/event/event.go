package event

import sharedevent "github.com/FangcunMount/iam/pkg/event"

const (
	SourceAPIServer = "iam-apiserver"
)

type DomainEvent = sharedevent.DomainEvent
type Stager = sharedevent.Stager
type Publisher = sharedevent.Publisher
type BaseEvent = sharedevent.BaseEvent
type Event[T any] = sharedevent.Event[T]

func NewBaseEvent(eventType, aggregateType, aggregateID string) BaseEvent {
	return sharedevent.NewBaseEvent(eventType, aggregateType, aggregateID)
}

func New[T any](eventType, aggregateType, aggregateID string, data T) Event[T] {
	return sharedevent.New(eventType, aggregateType, aggregateID, data)
}
