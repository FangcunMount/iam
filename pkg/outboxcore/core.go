package outboxcore

import (
	"fmt"
	"time"

	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
)

const StatusPending = "pending"

type Record struct {
	EventID       string
	EventType     string
	AggregateType string
	AggregateID   string
	TopicName     string
	PayloadJSON   string
	Status        string
	AttemptCount  int
	NextAttemptAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type BuildRecordsOptions struct {
	Events   []event.DomainEvent
	Resolver eventcatalog.TopicResolver
	Delivery eventcatalog.DeliveryClassResolver
	Now      time.Time
}

func BuildRecords(opts BuildRecordsOptions) ([]Record, error) {
	if len(opts.Events) == 0 {
		return nil, nil
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = eventcatalog.NewCatalog(nil)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	records := make([]Record, 0, len(opts.Events))
	for _, evt := range opts.Events {
		topicName, ok := resolver.GetTopicForEvent(evt.EventType())
		if !ok {
			return nil, fmt.Errorf("event %q not found in event catalog", evt.EventType())
		}
		if opts.Delivery != nil {
			delivery, ok := opts.Delivery.GetDeliveryClass(evt.EventType())
			if !ok {
				return nil, fmt.Errorf("event %q has no delivery class", evt.EventType())
			}
			if delivery != eventcatalog.DeliveryClassDurableOutbox {
				return nil, fmt.Errorf("event %q delivery class %q cannot be staged to outbox", evt.EventType(), delivery)
			}
		}
		payload, err := eventcodec.EncodePayload(evt)
		if err != nil {
			return nil, err
		}
		records = append(records, Record{
			EventID:       evt.EventID(),
			EventType:     evt.EventType(),
			AggregateType: evt.AggregateType(),
			AggregateID:   evt.AggregateID(),
			TopicName:     topicName,
			PayloadJSON:   string(payload),
			Status:        StatusPending,
			AttemptCount:  0,
			NextAttemptAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	return records, nil
}
