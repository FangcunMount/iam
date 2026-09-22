package eventoutbox

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/outbox"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReliableStager stores intents and fingerprints in the caller's original
// transaction. The caller must roll back that transaction on any Stage error.
// Historical rows retain their original creation timestamp on duplicate append.
type ReliableStager struct {
	builder     *Store
	topic       string
	clockOffset time.Duration
}

var _ event.Stager = (*ReliableStager)(nil)

func NewReliableStager(catalog *eventcatalog.Catalog, clockOffset time.Duration) (*ReliableStager, error) {
	if catalog == nil || !validHistoricalClock(clockOffset) {
		return nil, errors.New("policy event catalog required")
	}
	topic, ok := catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	if !ok || topic == "" || !catalog.IsDurableOutbox(eventing.AuthzVersionChanged) {
		return nil, errors.New("durable policy route required")
	}
	return &ReliableStager{builder: NewStore(nil, catalog), topic: topic, clockOffset: clockOffset}, nil
}
func (s *ReliableStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	if tx.Dialector == nil || tx.Dialector.Name() != "mysql" {
		return errors.New("historical reliable Stage requires MySQL")
	}
	for _, evt := range events {
		if evt == nil {
			return errors.New("nil domain event")
		}
	}
	rows, err := s.builder.buildRowsAt(events, time.Now().UTC().Truncate(time.Millisecond))
	if err != nil {
		return err
	}
	for _, row := range rows {
		_, e := policyIntent(*row, s.topic)
		if e != nil {
			return e
		}
		// Install the digest after reading the stored DATETIME digits: a MySQL
		// driver's loc setting can transform input times during parameter encoding.
		// Insert, read and digest update still belong to the same host transaction.
		candidate := map[string]any{
			"event_id": row.EventID, "event_type": row.EventType, "aggregate_type": row.AggregateType,
			"aggregate_id": row.AggregateID, "topic_name": row.TopicName, "payload_json": row.PayloadJSON,
			"status": row.Status, "attempt_count": row.AttemptCount,
			"next_attempt_at": historicalClockNow(s.clockOffset),
			"created_at":      historicalClockNow(s.clockOffset), "updated_at": historicalClockNow(s.clockOffset),
		}
		// The no-op duplicate update obtains the unique-index lock without replacing
		// any historical content, state, retry count, timestamp or publication receipt.
		if e = tx.WithContext(ctx).Model(&reliableRow{}).Clauses(clause.OnConflict{DoUpdates: clause.Assignments(map[string]any{"event_id": gorm.Expr("event_id")})}).Create(candidate).Error; e != nil {
			return e
		}
		var existing reliableRow
		if e = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id=?", row.EventID).First(&existing).Error; e != nil {
			return e
		}
		original, e := policyIntent(existing.OutboxPO, s.topic)
		if e != nil {
			return outbox.ErrConflict
		}
		originalFingerprint := original.Fingerprint()
		if existing.Fingerprint != nil && !bytes.Equal(existing.Fingerprint, originalFingerprint[:]) {
			return outbox.ErrConflict
		}
		// The old schema has only a persistence time; a caller retry's current clock
		// must not invent a different occurrence time for that same historical intent.
		row.CreatedAt = existing.CreatedAt
		retry, e := policyIntent(*row, s.topic)
		if e != nil || retry.Fingerprint() != originalFingerprint {
			return outbox.ErrConflict
		}
		if existing.Fingerprint == nil {
			if e = tx.WithContext(ctx).Model(&OutboxPO{}).Where("id=?", existing.ID).UpdateColumn("rm_fingerprint", originalFingerprint[:]).Error; e != nil {
				return e
			}
		}
	}
	return nil
}
