package legacyoutbox

import (
	"context"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messageprotocol"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"time"
)

// StandardStager is a historical integration fixture, never a host writer.
type StandardStager struct{ mapper *eventing.PolicyIntentMapper }

func NewStandardStager(catalog *eventcatalog.Catalog) (*StandardStager, error) {
	mapper, err := eventing.NewPolicyIntentMapper(catalog)
	if err != nil {
		return nil, err
	}
	return &StandardStager{mapper: mapper}, nil
}

func (s *StandardStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	if tx.Dialector == nil || tx.Dialector.Name() != "mysql" {
		return errors.New("standard reliable Stage requires MySQL")
	}
	if err = messageprotocol.RequireOutbox(ctx, tx, true); err != nil {
		return err
	}
	appender, err := sdkmysql.BindGORM(tx)
	if err != nil {
		return err
	}
	for _, evt := range events {
		intent, err := s.mapper.Message(evt)
		if err != nil {
			return err
		}
		if err = appender.Append(ctx, intent, time.Now()); err != nil {
			return err
		}
	}
	return nil
}
