//go:build reliable_messaging

package authz

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messagefailure"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	_ "github.com/go-sql-driver/mysql"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
)

// This test uses a dedicated empty MySQL database and a disposable nsqd. The
// first subscriber refuses durable audit, then a replacement with a different
// ephemeral business channel records the retained stable-group handoff. This
// exercises two instances in one test process, not an OS process restart.
func TestSDKPolicySyncFailureHandoffAcrossEphemeralChannels(t *testing.T) {
	if os.Getenv("RM_IAM_M6_REQUIRED") != "1" {
		t.Skip("dedicated MySQL and NSQ required")
	}
	dsn, tcp, httpURL := os.Getenv("RM_IAM_M6_MYSQL_DSN"), os.Getenv("RM_IAM_NSQ_TCP"), os.Getenv("RM_IAM_NSQ_HTTP")
	require.NotEmpty(t, dsn)
	require.NotEmpty(t, tcp)
	require.NotEmpty(t, httpURL)
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	require.NoError(t, db.Ping())
	var tables int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&tables))
	require.Zero(t, tables, "dedicated empty MySQL database required")
	ddl, err := os.ReadFile("../../../pkg/migration/migrations/000040_iam_nsq_failure_audit.up.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(ddl))
	require.NoError(t, err)
	audit, err := messagefailure.New(db)
	require.NoError(t, err)

	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 5 * time.Second}, []string{httpURL})
	require.NoError(t, err)
	newSubscriber := func() *sdknsq.Subscriber {
		config := nsq.NewConfig()
		config.DialTimeout, config.ReadTimeout, config.WriteTimeout = time.Second, 60*time.Second, time.Second
		subscriber, err := sdknsq.NewSubscriber(sdknsq.SubscriberConfig{
			NSQDAddresses: []string{tcp}, Driver: config, MaxInFlight: 1, MaxAttempts: 1,
			Retry:              sdknsq.Backoff{BaseDelay: 20 * time.Millisecond, MaxDelay: 100 * time.Millisecond},
			FailedHandoffGroup: ChannelPrefix,
		})
		require.NoError(t, err)
		return subscriber
	}
	makeSync := func(channel string, failed func(context.Context, legacy.FailedHandoff) error) *policySyncSubscriber {
		module := &AuthzModule{policyReloader: failingPolicyReloader{}}
		sync := module.SDKPolicySyncSubscriber(newSubscriber(), provisioner, failed)
		require.NotNil(t, sync)
		sync.channel = channel
		return sync
	}
	var denied atomic.Int32
	firstChannel := InstanceChannel("first-process", 1001)
	first := makeSync(firstChannel, func(context.Context, legacy.FailedHandoff) error {
		denied.Add(1)
		return errors.New("MySQL audit temporarily unavailable")
	})
	require.NoError(t, first.Start(context.Background()))
	require.True(t, first.registered, "business and failure consumers must both register before first publish")
	producer, err := nsq.NewProducer(tcp, nsq.NewConfig())
	require.NoError(t, err)
	defer producer.Stop()
	body, err := legacy.Encode(legacy.Envelope{
		UUID: "iam-sdk-failed-application-1", Metadata: map[string]string{"event_type": "iam.authz.version_changed.v2"},
		Payload: []byte(`{"version":12}`),
	}, legacy.Revision2)
	require.NoError(t, err)
	require.NoError(t, producer.Publish("iam.authz.version.v2", body))
	require.Eventually(t, func() bool { return denied.Load() > 0 }, 10*time.Second, 20*time.Millisecond)
	var before int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM iam_nsq_failure_audit").Scan(&before))
	require.Zero(t, before, "failed audit must not be counted as durable completion")
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	require.NoError(t, first.StopWithContext(stopCtx))
	cancel()

	second := makeSync(InstanceChannel("replacement-process", 1002), audit.Record)
	require.NoError(t, second.Start(context.Background()))
	require.True(t, second.registered, "replacement must attach to retained failure channel")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, second.StopWithContext(ctx))
	}()
	var applicationID, sourceChannel, transportID string
	var count int
	require.Eventually(t, func() bool {
		return db.QueryRow(`SELECT application_id,channel_name,first_transport_id FROM iam_nsq_failure_audit LIMIT 1`).Scan(
			&applicationID, &sourceChannel, &transportID,
		) == nil
	}, 10*time.Second, 20*time.Millisecond)
	require.Equal(t, "iam-sdk-failed-application-1", applicationID)
	require.Equal(t, firstChannel, sourceChannel)
	require.NotEmpty(t, transportID)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM iam_nsq_failure_audit").Scan(&count))
	require.Equal(t, 1, count)
}

type failingPolicyReloader struct{}

func (failingPolicyReloader) LoadPolicy(context.Context) error {
	return errors.New("controlled policy reload failure")
}
