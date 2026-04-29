package outboxcore_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/outboxcore"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
)

func testCatalog(t *testing.T) *eventcatalog.Catalog {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`
version: "1"
topics:
  authz_version:
    name: iam.authz.version
events:
  iam.authz.version_changed:
    topic: authz_version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
  iam.login_otp_sms:
    topic: authz_version
    delivery: best_effort
    aggregate: LoginOTP
    domain: authn
    handler: sms-dispatcher
`))
	require.NoError(t, err)
	return eventcatalog.NewCatalog(cfg)
}

func TestBuildRecordsAcceptsOnlyDurableOutboxEvents(t *testing.T) {
	catalog := testCatalog(t)
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	evt := event.New(eventcatalog.AuthzVersionChanged, "PolicyVersion", "tenant-a:2", map[string]any{
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
	require.Equal(t, eventcatalog.AuthzVersionChanged, records[0].EventType)
	require.Equal(t, "iam.authz.version", records[0].TopicName)
	require.Equal(t, outboxcore.StatusPending, records[0].Status)
	require.Equal(t, now, records[0].NextAttemptAt)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(records[0].PayloadJSON), &payload))
	require.Equal(t, "tenant-a", payload["tenant_id"])
	require.Equal(t, float64(2), payload["version"])

	bestEffort := event.New(eventcatalog.LoginOTPSMS, "LoginOTP", "+8613800138000", map[string]string{"code": "123456"})
	_, err = outboxcore.BuildRecords(outboxcore.BuildRecordsOptions{
		Events:   []event.DomainEvent{bestEffort},
		Resolver: catalog,
		Delivery: catalog,
		Now:      now,
	})
	require.ErrorContains(t, err, "cannot be staged to outbox")
}
