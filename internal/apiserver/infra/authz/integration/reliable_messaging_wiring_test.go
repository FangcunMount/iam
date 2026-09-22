//go:build reliable_messaging

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
)

func TestReliableMessagingPlatformWiring(t *testing.T) {
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	require.NotEmpty(t, os.Getenv("RM_IAM_NSQ_TCP"))
	db := authzdb.Open(t, true)
	require.NoError(t, db.Exec("DROP TABLE domain_event_outbox").Error)
	for _, variable := range []string{"RM_IAM_OUTBOX_SCHEMA", "RM_IAM_OUTBOX_UPGRADE"} {
		ddl, err := os.ReadFile(os.Getenv(variable))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(ddl)).Error)
	}
	opts := options.DefaultReliableMessagingOptions()
	opts.Enabled = true
	deps := platform.EventingDeps{DB: db, EventBus: wiringBus{}, CatalogPath: os.Getenv("RM_IAM_EVENTS_CATALOG"),
		ReliableMessaging: opts, NSQEnabled: true, NSQAddress: os.Getenv("RM_IAM_NSQ_TCP"), OutboxInterval: 10 * time.Millisecond}
	require.NotEmpty(t, deps.CatalogPath)
	_, err := platform.InitEventing(deps)
	require.Error(t, err, "missing migration journal must fail before allocating producer")
	// This fixture records the already installed exact schema; full migrator
	// correctness is separately proven by the complete migration/bootstrap test.
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT NOT NULL PRIMARY KEY, dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(38,TRUE)").Error)
	_, err = platform.InitEventing(deps)
	require.ErrorContains(t, err, "clean migration")
	require.NoError(t, db.Exec("UPDATE schema_migrations SET dirty=FALSE").Error)
	platformEventing, err := platform.InitEventing(deps)
	require.NoError(t, err)
	require.Nil(t, platformEventing.Relay, "legacy Relay must not be selected alongside SDK")
	require.IsType(t, &eventoutbox.StandardStager{}, platformEventing.Stager)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		require.NoError(t, platformEventing.ReliableRuntime.Stop(ctx))
		platformEventing.CloseReliableProducer()
	})
	topic, ok := platformEventing.Catalog.GetTopicForEvent("iam.authz.version_changed.v2")
	require.True(t, ok)
	consumer, err := nsq.NewConsumer(topic, "m3-wiring", nsq.NewConfig())
	require.NoError(t, err)
	consumer.SetLogger(nil, nsq.LogLevelError)
	received := make(chan []byte, 1)
	consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		select {
		case received <- append([]byte(nil), m.Body...):
		default:
		}
		return nil
	}))
	require.NoError(t, consumer.ConnectToNSQD(deps.NSQAddress))
	t.Cleanup(func() {
		consumer.Stop()
		select {
		case <-consumer.StopChan:
		case <-time.After(5 * time.Second):
			t.Error("consumer did not stop")
		}
	})
	evt := policy.NewVersionChangedEvent(2)
	uow := authzuow.NewUnitOfWork(db, nil, platformEventing.Stager)
	require.NoError(t, uow.WithinTx(context.Background(), func(ctx context.Context, repos appuow.TxRepositories) error {
		if _, err := repos.PolicyVersions.Increment(ctx, "m3", "wiring"); err != nil {
			return err
		}
		return repos.Events.Stage(ctx, evt)
	}))
	var before []byte
	require.NoError(t, db.Raw("SELECT payload FROM rm_outbox WHERE message_id=?", evt.EventID()).Row().Scan(&before))
	disabled := deps
	disabled.ReliableMessaging.Enabled = false
	_, err = platform.InitEventing(disabled)
	require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff, "cannot disable SDK with pending standard work")
	eventOptions := *options.NewEventOptions()
	eventOptions.CatalogPath = deps.CatalogPath
	blockedContainer := container.NewContainerWithOptions(db, nil, nil, nil, container.RuntimeOptions{Events: eventOptions})
	require.ErrorIs(t, blockedContainer.Initialize(), eventoutbox.ErrUnsafeMessagingHandoff, "container must preserve the cause for the process degraded-startup gate")
	require.Nil(t, blockedContainer.AuthzModule, "unsafe handoff must stop bootstrap before initializing other modules")
	snapshot, err := platformEventing.Outbox.OutboxStatusSnapshot(context.Background(), time.Now())
	require.NoError(t, err)
	require.Len(t, snapshot.Buckets, 1)
	require.Equal(t, "standard_pending", snapshot.Buckets[0].Status)
	require.EqualValues(t, 1, snapshot.Buckets[0].Count)
	require.NoError(t, platformEventing.ReliableRuntime.Start(context.Background()))
	select {
	case body := <-received:
		require.Equal(t, before, body, "actual NSQ receives original wire bytes")
	case <-time.After(10 * time.Second):
		t.Fatal("candidate publisher did not deliver")
	}
	require.Eventually(t, func() bool {
		var state string
		return db.Raw("SELECT state FROM rm_outbox WHERE message_id=?", evt.EventID()).Row().Scan(&state) == nil && state == "published"
	}, 5*time.Second, 10*time.Millisecond)
	var oldCount int64
	require.NoError(t, db.Table("domain_event_outbox").Count(&oldCount).Error)
	require.Zero(t, oldCount)
	// Unknown legacy work must block SDK startup, without changing the row.
	require.NoError(t, db.Exec(`INSERT INTO domain_event_outbox(event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,next_attempt_at) VALUES('old-pending','unknown','unknown','unknown','unknown','{}','pending',CURRENT_TIMESTAMP(3))`).Error)
	_, err = platform.InitEventing(deps)
	require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff)
	// Receiving bytes and published state are transport evidence. Full policy
	// subscriber topology, business decisions and crash recovery remain M3 gates.
}

type wiringBus struct{}

func (wiringBus) Publisher() cbmessaging.Publisher   { return nil }
func (wiringBus) Subscriber() cbmessaging.Subscriber { return nil }
func (wiringBus) Router() *cbmessaging.Router        { return nil }
func (wiringBus) Health() error                      { return nil }
func (wiringBus) Close() error                       { return nil }
