package eventoutbox

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/outboxcore"
	outboxport "github.com/FangcunMount/iam/internal/apiserver/port/outbox"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const storeName = "iam-mysql-outbox"

type OutboxPO struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	EventID       string     `gorm:"column:event_id;size:64;not null;uniqueIndex:uk_event_id"`
	EventType     string     `gorm:"column:event_type;size:128;not null"`
	AggregateType string     `gorm:"column:aggregate_type;size:64;not null"`
	AggregateID   string     `gorm:"column:aggregate_id;size:64;not null"`
	TopicName     string     `gorm:"column:topic_name;size:128;not null"`
	PayloadJSON   string     `gorm:"column:payload_json;type:longtext;not null"`
	Status        string     `gorm:"column:status;size:32;not null;index:idx_status_next_attempt_at,priority:1"`
	AttemptCount  int        `gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;not null;index:idx_status_next_attempt_at,priority:2"`
	LastError     *string    `gorm:"column:last_error;type:text"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
}

func (OutboxPO) TableName() string {
	return "domain_event_outbox"
}

type Store struct {
	db                 *gorm.DB
	publishingStaleFor time.Duration
	topicResolver      eventcatalog.TopicResolver
	deliveryResolver   eventcatalog.DeliveryClassResolver
}

var _ event.Stager = (*Store)(nil)
var _ outboxport.Store = (*Store)(nil)
var _ outboxport.StatusReader = (*Store)(nil)

func NewStore(db *gorm.DB, catalog *eventcatalog.Catalog) *Store {
	var topicResolver eventcatalog.TopicResolver = eventcatalog.NewCatalog(nil)
	var deliveryResolver eventcatalog.DeliveryClassResolver
	if catalog != nil {
		topicResolver = catalog
		deliveryResolver = catalog
	}
	return &Store{
		db:                 db,
		publishingStaleFor: outboxcore.DefaultPublishingStaleFor,
		topicResolver:      topicResolver,
		deliveryResolver:   deliveryResolver,
	}
}

func (s *Store) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	rows, err := s.buildRows(events)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&rows).Error
}

func (s *Store) buildRows(events []event.DomainEvent) ([]*OutboxPO, error) {
	return s.buildRowsAt(events, time.Now())
}

func (s *Store) buildRowsAt(events []event.DomainEvent, now time.Time) ([]*OutboxPO, error) {
	records, err := outboxcore.BuildRecords(outboxcore.BuildRecordsOptions{
		Events:   events,
		Resolver: s.topicResolver,
		Delivery: s.deliveryResolver,
		Now:      now,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]*OutboxPO, 0, len(records))
	for _, record := range records {
		rows = append(rows, &OutboxPO{
			EventID:       record.EventID,
			EventType:     record.EventType,
			AggregateType: record.AggregateType,
			AggregateID:   record.AggregateID,
			TopicName:     record.TopicName,
			PayloadJSON:   record.PayloadJSON,
			Status:        record.Status,
			AttemptCount:  record.AttemptCount,
			NextAttemptAt: record.NextAttemptAt,
			CreatedAt:     record.CreatedAt,
			UpdatedAt:     record.UpdatedAt,
		})
	}
	return rows, nil
}

func (s *Store) ClaimDueEvents(ctx context.Context, limit int, now time.Time) ([]outboxport.PendingEvent, error) {
	if s == nil || s.db == nil || limit <= 0 {
		return nil, nil
	}
	var rows []*OutboxPO
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		staleBefore := now.Add(-s.publishingStaleFor)
		query := tx
		if tx.Dialector == nil || tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := query.Where(
			"(status = ? AND next_attempt_at <= ?) OR (status = ? AND next_attempt_at <= ?) OR (status = ? AND updated_at <= ?)",
			outboxcore.StatusPending, now,
			outboxcore.StatusFailed, now,
			outboxcore.StatusPublishing, staleBefore,
		).
			Order("created_at ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return tx.Model(&OutboxPO{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":     outboxcore.StatusPublishing,
				"updated_at": now,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	claimed := make([]outboxport.PendingEvent, 0, len(rows))
	for _, row := range rows {
		claimed = append(claimed, outboxport.PendingEvent{
			EventID:       row.EventID,
			EventType:     row.EventType,
			AggregateType: row.AggregateType,
			AggregateID:   row.AggregateID,
			TopicName:     row.TopicName,
			Payload:       []byte(row.PayloadJSON),
		})
	}
	return claimed, nil
}

func (s *Store) MarkEventPublished(ctx context.Context, eventID string, publishedAt time.Time) error {
	return s.db.WithContext(ctx).Model(&OutboxPO{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{
			"status":       outboxcore.StatusPublished,
			"published_at": publishedAt,
			"updated_at":   publishedAt,
		}).Error
}

func (s *Store) MarkEventFailed(ctx context.Context, eventID, lastError string, nextAttemptAt time.Time) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&OutboxPO{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{
			"status":          outboxcore.StatusFailed,
			"last_error":      lastError,
			"next_attempt_at": nextAttemptAt,
			"updated_at":      now,
			"attempt_count":   gorm.Expr("attempt_count + ?", 1),
		}).Error
}

func (s *Store) OutboxStatusSnapshot(ctx context.Context, now time.Time) (outboxport.StatusSnapshot, error) {
	if s == nil || s.db == nil {
		return outboxcore.BuildStatusSnapshot(storeName, now, nil), nil
	}
	observations := make([]outboxcore.StatusObservation, 0, len(outboxcore.UnfinishedStatuses()))
	for _, status := range outboxcore.UnfinishedStatuses() {
		var count int64
		if err := s.db.WithContext(ctx).Model(&OutboxPO{}).Where("status = ?", status).Count(&count).Error; err != nil {
			return outboxport.StatusSnapshot{}, err
		}
		var oldestCreatedAt *time.Time
		if count > 0 {
			var oldest OutboxPO
			if err := s.db.WithContext(ctx).Where("status = ?", status).Order("created_at ASC").Limit(1).Find(&oldest).Error; err != nil {
				return outboxport.StatusSnapshot{}, err
			}
			oldestCreatedAt = &oldest.CreatedAt
		}
		observations = append(observations, outboxcore.StatusObservation{
			Status:          status,
			Count:           count,
			OldestCreatedAt: oldestCreatedAt,
		})
	}
	return outboxcore.BuildStatusSnapshot(storeName, now, observations), nil
}
