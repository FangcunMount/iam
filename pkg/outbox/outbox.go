package outbox

import (
	"context"
	"time"

	rmoutbox "github.com/FangcunMount/reliable-messaging/outbox"
)

// These public types retain IAM's Go type identity for existing callers.
// ToSDK converts the legacy public boundary to the shared messaging contract.
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

func (s StatusSnapshot) ToSDK() rmoutbox.StatusSnapshot {
	result := rmoutbox.StatusSnapshot{Store: s.Store, GeneratedAt: s.GeneratedAt}
	if s.Buckets != nil {
		result.Buckets = make([]rmoutbox.StatusBucket, 0, len(s.Buckets))
	}
	for _, bucket := range s.Buckets {
		result.Buckets = append(result.Buckets, rmoutbox.StatusBucket{
			Status: bucket.Status, Count: bucket.Count,
			OldestCreatedAt:  bucket.OldestCreatedAt,
			OldestAgeSeconds: bucket.OldestAgeSeconds,
		})
	}
	return result
}
