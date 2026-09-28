package outboxcore_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/outboxcore"
	"github.com/stretchr/testify/require"
)

const (
	testDurableEvent    = "test.version_changed"
	testBestEffortEvent = "test.notify"
)

func testCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
version: "1"
topics:
  version:
    name: test.version
events:
  test.version_changed:
    topic: version
    delivery: durable_outbox
    aggregate: Version
    domain: test
    handler: test-version-sync
  test.notify:
    topic: version
    delivery: best_effort
    aggregate: Notification
    domain: test
    handler: test-notifier
`))
	require.NoError(t, err)
	return eventcatalog.NewCatalog(cfg)
}

func TestBuildRecordsAcceptsOnlyDurableOutboxEvents(t *testing.T) {
	catalog := testCatalog(t)
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	evt := event.New(testDurableEvent, "Version", "tenant-a:2", map[string]any{
		"tenant_id": "tenant-a",
		"version":   2,
	})
	records, err := outboxcore.BuildRecords(outboxcore.BuildRecordsOptions{
		Events:   []event.DomainEvent{evt},
		Resolver: catalog,
		Delivery: catalog,
		Now:      now,
	})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, testDurableEvent, records[0].EventType)
	require.Equal(t, "test.version", records[0].TopicName)
	require.Equal(t, outboxcore.StatusPending, records[0].Status)
	require.Equal(t, now, records[0].NextAttemptAt)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(records[0].PayloadJSON), &payload))
	require.Equal(t, "tenant-a", payload["tenant_id"])
	require.Equal(t, float64(2), payload["version"])

	bestEffort := event.New(testBestEffortEvent, "Notification", "n-1", map[string]string{"message": "hello"})
	_, err = outboxcore.BuildRecords(outboxcore.BuildRecordsOptions{
		Events:   []event.DomainEvent{bestEffort},
		Resolver: catalog,
		Delivery: catalog,
		Now:      now,
	})
	require.ErrorContains(t, err, "cannot be staged to outbox")
}

func TestOutboxStatusJSONContractAcrossLegacyAndStandardReaders(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	oldest := now.Add(-2 * time.Minute)
	snapshot := outboxcore.BuildStatusSnapshot("iam-legacy-outbox", now, []outboxcore.StatusObservation{{
		Status: outboxcore.StatusPending, Count: 2, OldestCreatedAt: &oldest,
	}})
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	const want = `{"store":"iam-legacy-outbox","generated_at":"2026-09-28T10:00:00+08:00","buckets":[{"status":"pending","count":2,"oldest_created_at":"2026-09-28T09:58:00+08:00","oldest_age_seconds":120},{"status":"failed","count":0,"oldest_age_seconds":0},{"status":"publishing","count":0,"oldest_age_seconds":0},{"status":"quarantined","count":0,"oldest_age_seconds":0}]}`
	require.JSONEq(t, want, string(encoded))
}
