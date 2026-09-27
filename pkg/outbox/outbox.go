package outbox

import (
	"context"
	"time"

	rmoutbox "github.com/FangcunMount/reliable-messaging/outbox"
)

type PendingEvent struct {
	EventID       string
	EventType     string
	AggregateType string
	AggregateID   string
	TopicName     string
	Payload       []byte
}

type Store interface {
	ClaimDueEvents(ctx context.Context, limit int, now time.Time) ([]PendingEvent, error)
	MarkEventPublished(ctx context.Context, eventID string, publishedAt time.Time) error
	MarkEventFailed(ctx context.Context, eventID, lastError string, nextAttemptAt time.Time) error
}

// Status is a shared messaging observation contract. IAM still owns the
// database read, the legacy/standard state mapping and readiness policy.
type StatusBucket = rmoutbox.StatusBucket
type StatusSnapshot = rmoutbox.StatusSnapshot
type StatusReader = rmoutbox.StatusReader
