package container

import (
	"context"
	"strings"
	"testing"
	"time"

	eventoutbox "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	"github.com/FangcunMount/iam/v3/pkg/outboxcore"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDomainEventOutboxReadinessRejectsStaleBacklog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&eventoutbox.OutboxPO{}))

	now := time.Now().UTC()
	require.NoError(t, db.Create(&eventoutbox.OutboxPO{
		EventID:       "stale-event",
		EventType:     "iam.test",
		AggregateType: "Test",
		AggregateID:   "1",
		TopicName:     "iam.test",
		PayloadJSON:   `{}`,
		Status:        outboxcore.StatusPending,
		NextAttemptAt: now.Add(-10 * time.Minute),
		CreatedAt:     now.Add(-10 * time.Minute),
		UpdatedAt:     now.Add(-10 * time.Minute),
	}).Error)

	container := &Container{
		outboxStore: eventoutbox.NewStore(db, nil),
		runtimeOptions: RuntimeOptions{Health: apiserveroptions.HealthOptions{
			Readiness: apiserveroptions.ReadinessOptions{OutboxMaxPendingAge: 5 * time.Minute},
		}},
	}
	require.ErrorContains(t, container.checkDomainEventOutboxReady(context.Background()), "backlog exceeded")
}

func TestDomainEventOutboxReadinessAcceptsEmptyStore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&eventoutbox.OutboxPO{}))

	container := &Container{
		outboxStore: eventoutbox.NewStore(db, nil),
		runtimeOptions: RuntimeOptions{Health: apiserveroptions.HealthOptions{
			Readiness: apiserveroptions.ReadinessOptions{OutboxMaxPendingAge: 5 * time.Minute},
		}},
	}
	require.NoError(t, container.checkDomainEventOutboxReady(context.Background()))
}

func TestReadinessRegistersDomainEventOutboxAsRequired(t *testing.T) {
	container := &Container{
		runtimeOptions: RuntimeOptions{Health: apiserveroptions.HealthOptions{
			Readiness: apiserveroptions.ReadinessOptions{
				ComponentTimeout:    time.Second,
				TotalTimeout:        2 * time.Second,
				OutboxMaxPendingAge: 5 * time.Minute,
			},
		}},
	}
	snapshot, ready := container.ReadinessChecker().Check(context.Background())

	require.False(t, ready)
	require.Equal(t, "failed", string(snapshot.Components["domain_event_outbox"].Status))
}
