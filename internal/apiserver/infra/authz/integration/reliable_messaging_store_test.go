//go:build reliable_messaging

package integration_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/reliable-messaging/outbox"
	"github.com/stretchr/testify/require"
)

func TestReliableMessagingHistoricalStore(t *testing.T) {
	if os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN") == "" {
		t.Fatal("isolated MySQL DSN required")
	}
	db := authzdb.Open(t, true)
	// Use the exact historical DDL instead of the fixture's AutoMigrate precision.
	require.NoError(t, db.Exec("DROP TABLE domain_event_outbox").Error)
	schema, err := os.ReadFile(os.Getenv("RM_IAM_OUTBOX_SCHEMA"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
	require.NoError(t, db.Exec(`ALTER TABLE domain_event_outbox
 ADD COLUMN rm_claim_token VARCHAR(64) NULL,
 ADD COLUMN rm_claim_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_claim_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
 ADD COLUMN rm_lease_until DATETIME(6) NULL,
 ADD COLUMN rm_fingerprint BINARY(32) NULL`).Error)
	ctx := context.Background()
	s, err := eventoutbox.NewReliableStore(db, "iam.authz.version.v2", time.Minute)
	require.NoError(t, err)
	row := eventoutbox.OutboxPO{EventID: "old-intent", EventType: "iam.authz.version_changed.v2", AggregateType: "PolicyVersion", AggregateID: "2", TopicName: "iam.authz.version.v2", PayloadJSON: `{ "version":2,"extension":true }`, Status: "failed", AttemptCount: 7, NextAttemptAt: time.Now().Add(-time.Hour), CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour)}
	require.NoError(t, db.Create(&row).Error)
	var wg sync.WaitGroup
	claims := make(chan []outbox.Claim, 2)
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); c, e := s.ClaimDue(ctx, 1, time.Minute); claims <- c; errors <- e }()
	}
	wg.Wait()
	close(claims)
	close(errors)
	for e := range errors {
		require.NoError(t, e)
	}
	var claimed []outbox.Claim
	for batch := range claims {
		claimed = append(claimed, batch...)
	}
	require.Len(t, claimed, 1)
	old := claimed[0]
	require.Equal(t, []byte(row.PayloadJSON), old.Message.Input().Payload)
	require.EqualValues(t, 1, old.Attempts)
	require.NoError(t, s.Retry(ctx, old, time.Nanosecond, "unknown"))
	var after eventoutbox.OutboxPO
	require.NoError(t, db.First(&after, row.ID).Error)
	require.Equal(t, 8, after.AttemptCount)
	require.Equal(t, "failed", after.Status)
	require.ErrorIs(t, s.Confirm(ctx, old), outbox.ErrStaleClaim)
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET next_attempt_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?", row.ID).Error)
	second, err := s.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.NoError(t, db.Exec("UPDATE domain_event_outbox SET rm_lease_until=UTC_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE id=?", row.ID).Error)
	third, err := s.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.ErrorIs(t, s.Confirm(ctx, second[0]), outbox.ErrStaleClaim)
	require.ErrorIs(t, s.Retry(ctx, second[0], time.Second, "late"), outbox.ErrStaleClaim)
	require.ErrorIs(t, s.Quarantine(ctx, second[0], "late"), outbox.ErrStaleClaim)
	require.NoError(t, s.Confirm(ctx, third[0]))
	require.NoError(t, db.First(&after, row.ID).Error)
	require.Equal(t, "published", after.Status)
	require.Equal(t, 8, after.AttemptCount)
	require.Equal(t, row.PayloadJSON, after.PayloadJSON)
	// A valid JSON edit under the same identity must not become a new delivery.
	tampered := row
	tampered.ID = 0
	tampered.EventID = "tampered-intent"
	require.NoError(t, db.Create(&tampered).Error)
	original, err := s.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, original, 1)
	require.NoError(t, s.Retry(ctx, original[0], time.Second, "unknown"))
	require.NoError(t, db.Exec(`UPDATE domain_event_outbox SET payload_json='{"version":2,"extension":false}', next_attempt_at=UTC_TIMESTAMP(3)-INTERVAL 1 SECOND WHERE id=?`, tampered.ID).Error)
	rejected, err := s.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, rejected)
	var mismatch eventoutbox.OutboxPO
	require.NoError(t, db.First(&mismatch, tampered.ID).Error)
	require.Equal(t, "quarantined", mismatch.Status)
	require.NotNil(t, mismatch.LastError)
	require.Equal(t, "historical_identity_conflict", *mismatch.LastError)
	require.Contains(t, mismatch.PayloadJSON, "false", "corrupt evidence retained")
	// Unsupported historical rows remain visible and cannot starve valid work.
	bad := row
	bad.ID = 0
	bad.EventID = "unknown-intent"
	bad.EventType = "unknown"
	require.NoError(t, db.Create(&bad).Error)
	none, err := s.ClaimDue(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, none)
	snapshot, err := eventoutbox.NewStore(db, nil).OutboxStatusSnapshot(ctx, time.Now())
	require.NoError(t, err)
	found := false
	for _, b := range snapshot.Buckets {
		if b.Status == "quarantined" {
			require.EqualValues(t, 2, b.Count)
			found = true
		}
	}
	require.True(t, found, "quarantine must be visible")
}
