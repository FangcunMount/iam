//go:build reliable_messaging

package integration_test

import (
	"context"
	"errors"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	role "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
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

func TestReliableMessagingHistoricalStager(t *testing.T) {
	if os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN") == "" {
		t.Fatal("isolated MySQL DSN required")
	}
	db := authzdb.Open(t, true)
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
	cfg, err := eventcatalog.Parse([]byte(`version: "1"
topics:
  version:
    name: iam.authz.version.v2
events:
  iam.authz.version_changed.v2:
    topic: version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
`))
	require.NoError(t, err)
	stager, err := eventoutbox.NewReliableStager(eventcatalog.NewCatalog(cfg))
	require.NoError(t, err)
	ctx := context.Background()
	original := policy.NewVersionChangedEvent(2)
	require.ErrorIs(t, stager.Stage(ctx, original), dbmysql.ErrActiveTransactionRequired)
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if _, e := repos.PolicyVersions.Increment(txctx, "rm-m3", "commit"); e != nil {
			return e
		}
		return repos.Events.Stage(txctx, original)
	}))
	var before eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id=?", original.EventID()).First(&before).Error)
	var hash []byte
	require.NoError(t, db.Raw("SELECT rm_fingerprint FROM domain_event_outbox WHERE event_id=?", original.EventID()).Row().Scan(&hash))
	require.Len(t, hash, 32)
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		return repos.Events.Stage(txctx, original)
	}))
	var after eventoutbox.OutboxPO
	require.NoError(t, db.Where("event_id=?", original.EventID()).First(&after).Error)
	require.Equal(t, before, after, "duplicate append must preserve the original row")
	changed := changedPolicyPayload{DomainEvent: original}
	businessRole, e := role.NewRole("rm-stage-conflict", "conflict")
	require.NoError(t, e)
	err = uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if e := repos.Roles.Create(txctx, &businessRole); e != nil {
			return e
		}
		if _, e := repos.PolicyVersions.Increment(txctx, "rm-m3", "conflict"); e != nil {
			return e
		}
		return repos.Events.Stage(txctx, changed)
	})
	require.ErrorIs(t, err, outbox.ErrConflict)
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM authz_roles WHERE name=?", "rm-stage-conflict").Scan(&count).Error)
	require.Zero(t, count)
	var version int64
	require.NoError(t, db.Raw("SELECT MAX(policy_version) FROM authz_policy_versions").Scan(&version).Error)
	require.EqualValues(t, 2, version)
	abort := errors.New("host rollback")
	next := policy.NewVersionChangedEvent(3)
	err = uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if e := repos.Events.Stage(txctx, next); e != nil {
			return e
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Where("event_id=?", next.EventID()).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

type changedPolicyPayload struct{ event.DomainEvent }

func (changedPolicyPayload) Payload() any {
	return map[string]any{"version": 2, "extension": "conflict"}
}
