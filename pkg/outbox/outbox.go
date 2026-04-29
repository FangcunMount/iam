package outbox

import (
	"context"
	"time"
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

type StatusBucket struct {
	Status           string     `json:"status"`
	Count            int64      `json:"count"`
	OldestCreatedAt  *time.Time `json:"oldest_created_at,omitempty"`
	OldestAgeSeconds float64    `json:"oldest_age_seconds"`
}

type StatusSnapshot struct {
	Store       string         `json:"store"`
	GeneratedAt time.Time      `json:"generated_at"`
	Buckets     []StatusBucket `json:"buckets"`
}

type StatusReader interface {
	OutboxStatusSnapshot(ctx context.Context, now time.Time) (StatusSnapshot, error)
}
