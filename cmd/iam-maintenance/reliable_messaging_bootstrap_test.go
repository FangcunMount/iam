//go:build reliable_messaging

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	_ "github.com/FangcunMount/component-base/pkg/messaging/nsq"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
)

// The proof script provides the complete, freshly migrated disposable database.
// These are sequential OS processes running actual messaging composition, not
// the complete API server or a simulation of production writer exclusion.
func TestMaintenanceBootstrapHandoff(t *testing.T) {
	require.Equal(t, "1", os.Getenv("IAM_RM_BOOTSTRAP_REQUIRED"))
	address := os.Getenv("RM_IAM_NSQ_TCP")
	require.NotEmpty(t, address)
	db, err := roleDatabase("IAM_APISERVER_")
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	var initial []eventoutbox.OutboxPO
	require.NoError(t, db.Order("id").Find(&initial).Error)
	require.Len(t, initial, 2, "original migration 33/35 notifications required")
	expected := make(map[string]string)
	for _, row := range initial {
		require.Equal(t, "pending", row.Status)
		expected[row.EventID] = row.PayloadJSON
	}
	var count int64
	require.NoError(t, db.Table("rm_outbox").Count(&count).Error)
	require.Zero(t, count)

	consumer, err := nsq.NewConsumer(initial[0].TopicName, "m3-bootstrap-handoff", nsq.NewConfig())
	require.NoError(t, err)
	consumer.SetLogger(nil, nsq.LogLevelError)
	received := make(chan []byte, 16)
	consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		select {
		case received <- append([]byte(nil), m.Body...):
		default:
		}
		return nil
	}))
	require.NoError(t, consumer.ConnectToNSQD(address))
	t.Cleanup(func() {
		consumer.Stop()
		select {
		case <-consumer.StopChan:
		case <-time.After(5 * time.Second):
			t.Error("bootstrap consumer did not stop")
		}
	})
	runPhase := func(phase string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		binary, err := os.Executable()
		require.NoError(t, err)
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestMaintenanceBootstrapHandoffChild$", "-test.v")
		cmd.Env = append(os.Environ(), "IAM_RM_HANDOFF_PHASE="+phase)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s process failed: %s", phase, output)
		require.NotNil(t, cmd.ProcessState)
		require.True(t, cmd.ProcessState.Exited())
		t.Logf("%s process exited successfully", phase)
	}
	await := func(want map[string]string) {
		t.Helper()
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		seen := make(map[string]bool)
		for len(seen) < len(want) {
			select {
			case body := <-received:
				decoded, ok, err := cbmessaging.DecodeMessagePayload(body)
				require.NoError(t, err)
				require.True(t, ok, "both transports must keep the existing envelope")
				payload, exists := want[decoded.UUID]
				require.True(t, exists, "unexpected event or replay from an earlier phase: %s", decoded.UUID)
				require.Equal(t, payload, string(decoded.Payload))
				require.Equal(t, "iam-outbox-relay", decoded.Metadata["source"])
				require.Equal(t, "iam.authz.version_changed.v2", decoded.Metadata["event_type"])
				seen[decoded.UUID] = true
			case <-timer.C:
				t.Fatal("bootstrap notification delivery timed out")
			}
		}
	}
	preflight := func(wantReady bool) {
		t.Helper()
		var output bytes.Buffer
		err := runReliableMessaging([]string{"preflight", "--target=sdk", "--event-catalog=" + os.Getenv("RM_IAM_EVENTS_CATALOG")}, &output)
		if wantReady {
			require.NoError(t, err, "%s", output.String())
		} else {
			require.Error(t, err)
		}
		var report eventoutbox.ReliablePreflightReport
		require.NoError(t, json.Unmarshal(output.Bytes(), &report))
		require.Equal(t, wantReady, report.SDKDataReady)
		require.False(t, report.CutoverAuthorized || report.WriterExclusionVerified)
	}
	preflight(false)
	runPhase("legacy-drain")
	await(expected)
	require.NoError(t, eventoutbox.CheckLegacyDrained(context.Background(), db))
	var retained []eventoutbox.OutboxPO
	require.NoError(t, db.Order("id").Find(&retained).Error)
	require.Len(t, retained, len(initial))
	for i, row := range retained {
		require.Equal(t, "published", row.Status)
		require.Equal(t, initial[i].EventID, row.EventID)
		require.Equal(t, initial[i].PayloadJSON, row.PayloadJSON)
		require.Equal(t, initial[i].CreatedAt, row.CreatedAt)
	}
	preflight(true)
	runPhase("standard")
	var standard struct {
		MessageID    string
		Payload      []byte
		State        string
		AttemptCount uint64
	}
	require.NoError(t, db.Raw("SELECT message_id,payload,state,attempt_count FROM rm_outbox").Scan(&standard).Error)
	require.Equal(t, "published", standard.State)
	await(map[string]string{standard.MessageID: string(standard.Payload)})
	runPhase("standard-resume")
	var attempts uint64
	require.NoError(t, db.Raw("SELECT attempt_count FROM rm_outbox").Scan(&attempts).Error)
	require.Equal(t, standard.AttemptCount, attempts, "a new process must not republish confirmed work")
	runPhase("legacy-rollback")
	var rollback eventoutbox.OutboxPO
	require.NoError(t, db.Order("id DESC").First(&rollback).Error)
	require.Equal(t, "published", rollback.Status)
	require.NotContains(t, expected, rollback.EventID)
	await(map[string]string{rollback.EventID: rollback.PayloadJSON})
	require.NoError(t, db.Table("domain_event_outbox").Count(&count).Error)
	require.EqualValues(t, 3, count)
	require.NoError(t, db.Table("rm_outbox").Count(&count).Error)
	require.EqualValues(t, 1, count)
	preflight(true)
}

func TestMaintenanceBootstrapHandoffChild(t *testing.T) {
	phase := os.Getenv("IAM_RM_HANDOFF_PHASE")
	if phase == "" {
		return // Parent launches this helper explicitly; no standalone fixture claim.
	}
	require.Equal(t, "1", os.Getenv("IAM_RM_BOOTSTRAP_REQUIRED"))
	require.Contains(t, []string{"legacy-drain", "standard", "standard-resume", "legacy-rollback"}, phase)
	db, err := roleDatabase("IAM_APISERVER_")
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer func() { require.NoError(t, pool.Close()) }()
	cfg := cbmessaging.DefaultConfig()
	cfg.NSQ.NSQdAddr = os.Getenv("RM_IAM_NSQ_TCP")
	bus, err := cbmessaging.NewEventBus(cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, bus.Close()) }()
	opts := options.DefaultReliableMessagingOptions()
	opts.Enabled = phase == "standard" || phase == "standard-resume"
	owner, err := platform.InitEventing(platform.EventingDeps{DB: db, EventBus: bus,
		CatalogPath: os.Getenv("RM_IAM_EVENTS_CATALOG"), NSQEnabled: true, NSQAddress: cfg.NSQ.NSQdAddr,
		ReliableMessaging: opts, OutboxInterval: 10 * time.Millisecond})
	require.NoError(t, err)
	if owner.ReliableRuntime != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, owner.ReliableRuntime.Stop(ctx))
			owner.CloseReliableProducer()
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if phase == "standard" || phase == "legacy-rollback" {
		uow := authzuow.NewUnitOfWork(db, nil, owner.Stager)
		require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
			version, err := repos.PolicyVersions.Increment(txctx, "isolated-bootstrap-proof", phase)
			if err != nil {
				return err
			}
			return repos.Events.Stage(txctx, policy.NewVersionChangedEvent(version.Version))
		}))
	}
	if owner.Relay != nil {
		require.Nil(t, owner.ReliableRuntime)
		require.NoError(t, owner.Relay.DispatchDue(ctx))
		require.NoError(t, eventoutbox.CheckLegacyDrained(ctx, db))
		return
	}
	require.Nil(t, owner.Relay)
	require.NoError(t, owner.ReliableRuntime.Start(ctx))
	if phase == "standard" {
		require.Eventually(t, func() bool { return eventoutbox.CheckStandardDrained(ctx, db) == nil }, 10*time.Second, 10*time.Millisecond)
	} else {
		// Wait for several real polling intervals in a new process. Stable claim
		// counts plus the next phase's wire checks detect accidental republishing.
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
