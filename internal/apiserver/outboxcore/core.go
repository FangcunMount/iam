package outboxcore

import (
	"fmt"
	"time"

	outboxport "github.com/FangcunMount/iam/internal/apiserver/port/outbox"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/FangcunMount/iam/internal/pkg/eventcodec"
)

const (
	StatusPending    = "pending"
	StatusPublishing = "publishing"
	StatusPublished  = "published"
	StatusFailed     = "failed"

	DefaultPublishingStaleFor      = time.Minute
	DefaultRelayRetryDelay         = 10 * time.Second
	DefaultDecodeFailureRetryDelay = 10 * time.Second
)

var unfinishedStatuses = []string{StatusPending, StatusFailed, StatusPublishing}

func UnfinishedStatuses() []string {
	return append([]string(nil), unfinishedStatuses...)
}

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

type StatusObservation struct {
	Status          string
	Count           int64
	OldestCreatedAt *time.Time
}

func BuildStatusSnapshot(store string, now time.Time, observations []StatusObservation) outboxport.StatusSnapshot {
	if now.IsZero() {
		now = time.Now()
	}
	byStatus := make(map[string]StatusObservation, len(observations))
	for _, observation := range observations {
		if isUnfinishedStatus(observation.Status) {
			byStatus[observation.Status] = observation
		}
	}
	buckets := make([]outboxport.StatusBucket, 0, len(unfinishedStatuses))
	for _, status := range unfinishedStatuses {
		observation := byStatus[status]
		ageSeconds := 0.0
		if observation.Count > 0 && observation.OldestCreatedAt != nil {
			ageSeconds = now.Sub(*observation.OldestCreatedAt).Seconds()
			if ageSeconds < 0 {
				ageSeconds = 0
			}
		}
		buckets = append(buckets, outboxport.StatusBucket{
			Status:           status,
			Count:            observation.Count,
			OldestCreatedAt:  observation.OldestCreatedAt,
			OldestAgeSeconds: ageSeconds,
		})
	}
	return outboxport.StatusSnapshot{Store: store, GeneratedAt: now, Buckets: buckets}
}

func isUnfinishedStatus(status string) bool {
	for _, known := range unfinishedStatuses {
		if status == known {
			return true
		}
	}
	return false
}
