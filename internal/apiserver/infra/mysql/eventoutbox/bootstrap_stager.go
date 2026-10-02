package eventoutbox

import (
	"context"
	"time"

	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
)

// BootstrapStager is the insert-only historical-schema adapter for fresh
// migration 33/35, before migration 38 creates the SDK table. It shares the
// current policy/SDK message mapper and has no delivery or recovery methods.
// Current runtime and maintenance must use StandardStager instead.
type BootstrapStager struct{ mapper *StandardStager }

var _ event.Stager = (*BootstrapStager)(nil)

func NewBootstrapStager(catalog *eventcatalog.Catalog) (*BootstrapStager, error) {
	mapper, err := NewStandardStager(catalog)
	if err != nil {
		return nil, err
	}
	return &BootstrapStager{mapper: mapper}, nil
}

func (s *BootstrapStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	rows := make([]*OutboxPO, 0, len(events))
	for _, evt := range events {
		intent, err := s.mapper.intent(evt)
		if err != nil {
			return err
		}
		input := intent.Input()
		rows = append(rows, &OutboxPO{
			EventID: input.ID, EventType: input.EventType,
			AggregateType: evt.AggregateType(), AggregateID: evt.AggregateID(),
			TopicName: input.Destination, PayloadJSON: string(input.Payload),
			Status: "pending", NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
		})
	}
	return tx.WithContext(ctx).Create(&rows).Error
}
