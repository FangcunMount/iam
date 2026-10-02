package container

import (
	"context"
	"testing"
	"time"

	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/outbox"
	"github.com/stretchr/testify/require"
)

// Readiness tests consume the status port. Real MySQL standard/legacy reading
// is covered by the isolated StandardPreflight contract, not a SQLite old Store.
type readinessOutboxSnapshot struct{ buckets []outbox.StatusBucket }

func (s readinessOutboxSnapshot) OutboxStatusSnapshot(_ context.Context, now time.Time) (outbox.StatusSnapshot, error) {
	return outbox.StatusSnapshot{Store: "iam-standard-and-legacy-outbox", GeneratedAt: now, Buckets: s.buckets}, nil
}

func TestDomainEventOutboxReadinessRejectsStaleBacklog(t *testing.T) {
	container := &Container{
		outboxStore: readinessOutboxSnapshot{buckets: []outbox.StatusBucket{{Status: "standard_pending", Count: 1, OldestAgeSeconds: (10 * time.Minute).Seconds()}}},
		runtimeOptions: RuntimeOptions{Health: apiserveroptions.HealthOptions{
			Readiness: apiserveroptions.ReadinessOptions{OutboxMaxPendingAge: 5 * time.Minute},
		}},
	}
	require.ErrorContains(t, container.checkDomainEventOutboxReady(context.Background()), "backlog exceeded")
}

func TestDomainEventOutboxReadinessAcceptsEmptyStore(t *testing.T) {
	container := &Container{
		outboxStore: readinessOutboxSnapshot{},
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
