package process

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	componentdb "github.com/FangcunMount/component-base/pkg/database"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/freshbootstrap"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/message"
	txmysql "github.com/FangcunMount/reliable-messaging/storage/transaction/mysql"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestF05BootstrapOwnerNativeMySQL(t *testing.T) {
	path := os.Getenv("IAM_F05_BROKER_FIXTURE")
	if path == "" {
		t.Skip("dedicated isolated MySQL and RocketMQ required")
	}
	require.Equal(t, "127.0.0.1", os.Getenv("MYSQL_HOST"))
	require.Equal(t, "iam_test", os.Getenv("MYSQL_DATABASE"))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular() && info.Mode().Perm() == 0600)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	type principal struct{ Username, Password string }
	var material struct {
		Endpoint, CAFile    string
		Publisher, Receiver principal
	}
	require.True(t, json.Unmarshal(raw, &material) == nil, "private Broker material invalid")
	host, _, err := net.SplitHostPort(material.Endpoint)
	require.True(t, err == nil && host == "127.0.0.1", "owned loopback Broker required")
	ca, err := os.ReadFile(material.CAFile)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}

	connector, err := componentdb.NewMySQLConnector(&componentdb.MySQLConfig{
		Host: "127.0.0.1:" + os.Getenv("MYSQL_PORT"), Username: "root",
		Password: os.Getenv("MYSQL_PASSWORD"), Database: "iam_test", Location: "Etc/GMT-8", SessionTimeZone: "+08:00",
	})
	require.NoError(t, err)
	pool := sql.OpenDB(connector)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	count := func(query string) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRowContext(ctx, query).Scan(&n))
		return n
	}
	require.Zero(t, count("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()"))
	readDB, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: pool}), &gorm.Config{})
	require.NoError(t, err)
	var consumerRuntime *authzruntime.Runtime
	migrator := migration.NewMigrator(pool, &migration.Config{Enabled: true, Database: "iam_test"})
	var serverUUID string
	require.NoError(t, pool.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&serverUUID))
	version, _, err := migrator.RunFreshForTransactionDelivery(ctx, serverUUID, func(conn *sql.Conn) (migration.FreshBootstrapHooks, error) {
		hooks, err := freshbootstrap.Bind(ctx, conn, readDB, "../../../configs/events.yaml", true)
		if err != nil {
			return hooks, err
		}
		apply := hooks.RetireConditions
		hooks.RetireConditions = func() error {
			// Snapshot the actual policy state before the second mutation.
			var err error
			consumerRuntime, err = authzruntime.NewRuntime(ctx, authzruntime.NewMySQLSource(readDB), authorization.NewEvaluator())
			if err != nil {
				return err
			}
			return apply()
		}
		return hooks, nil
	})
	require.NoError(t, err)
	require.EqualValues(t, 45, version)
	require.Zero(t, count("SELECT COUNT(*) FROM domain_event_outbox"))
	require.Zero(t, count("SELECT COUNT(*) FROM rm_outbox"))
	require.Equal(t, 2, count("SELECT COUNT(*) FROM iam_bootstrap_policy_notification WHERE delivery_committed=0"))

	const topic, physical = "iam.authz.version.v2", "iam-authz-version-v2--tx--iam-mysql--m3-1"
	const subscription, group = "iam-policy-sync.iam-primary-1", "iam-authz-version-v2--iam-policy-sync-iam-primary-1"
	binding := transaction.Binding{Owner: "iam.mysql", StoreID: "iam-f05-isolated", WriterEpoch: "proof-1"}
	_, err = pool.ExecContext(ctx, "INSERT INTO rm_tx_binding VALUES(?,?,?,1)", []byte(binding.Owner), []byte(binding.StoreID), []byte(binding.WriterEpoch))
	require.NoError(t, err)
	store, err := txmysql.New(pool, binding)
	require.NoError(t, err)
	_, err = pool.ExecContext(ctx, `UPDATE iam_authz_message_protocol SET protocol_type='transaction',store_id=?,writer_epoch=?,route_revision='m3-1',generation=2 WHERE owner='iam.mysql'`, binding.StoreID, binding.WriterEpoch)
	require.NoError(t, err)
	dir := t.TempDir()
	creds, routes := filepath.Join(dir, "publisher.json"), filepath.Join(dir, "routes.json")
	encoded, err := json.Marshal(material.Publisher)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(creds, encoded, 0600))
	route := transport.RouteBinding{Topic: topic, Subscription: subscription, PhysicalTopic: physical, ConsumerGroup: group, MessageType: transport.TransactionMessageType, Owner: binding.Owner, RouteRevision: "m3-1", NamingScheme: "readable-v1"}
	encoded, err = json.Marshal(map[string]any{"version": 3, "routes": []transport.RouteBinding{route}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(routes, encoded, 0600))
	catalog, err := eventcatalog.Load("../../../configs/events-m3.yaml")
	require.NoError(t, err)
	newOwner := func() *platform.TransactionEventing {
		t.Helper()
		owner, err := platform.NewTransactionEventing(platform.EventingDeps{DB: readDB, Messaging: options.MessagingOptions{
			Provider: "rocketmq", Endpoint: material.Endpoint, CAFile: material.CAFile, CredentialsFile: creds, RoutesFile: routes, InstanceID: "f05-native", PublishEnabled: true, RequireFreshBootstrap: true,
			Transactions: map[string]options.TransactionOptions{binding.Owner: {StoreID: binding.StoreID, WriterEpoch: binding.WriterEpoch, RouteRevision: "m3-1", Limits: &options.TransactionLimits{
				Sends: 2, Branches: 2, CheckWorkers: 2, CheckQueue: 4, ReconcileBatch: 6,
				CheckTimeout: 2 * time.Second, RPCTimeout: 5 * time.Second, Lease: 20 * time.Second, ReconcileInterval: 300 * time.Millisecond,
			}}},
		}}, eventcatalog.NewCatalog(catalog), "")
		require.True(t, err == nil, "fixture owner rejected configured route: %v", err)
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			require.NoError(t, owner.Stop(closeCtx))
		})
		return owner
	}
	subscriber, err := rocketmq.NewSubscriber(rocketmq.SubscriberConfig{
		Config: rocketmq.Config{
			TLS: tlsConfig, Endpoint: material.Endpoint, AccessKey: material.Receiver.Username,
			SecretKey: material.Receiver.Password, RequestTimeout: 5 * time.Second, MaxInFlight: 1,
			Routes: []transport.RouteBinding{{Topic: topic, Subscription: subscription, PhysicalTopic: physical,
				ConsumerGroup: group, MessageType: transport.TransactionMessageType, Owner: binding.Owner, RouteRevision: "m3-1", NamingScheme: "readable-v1"}},
		},
		MaxAttempts: 3, InvisibleDuration: 15 * time.Second, HandlerTimeout: 5 * time.Second, RetryDelay: time.Second,
	})
	require.True(t, err == nil, "fixture subscriber rejected")
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		require.NoError(t, subscriber.Close(closeCtx))
	}()
	service := policypublication.NewService(consumerRuntime, consumerRuntime)
	received := make(chan transport.Received, 4)
	require.NoError(t, subscriber.Subscribe(ctx, topic, subscription, func(ctx context.Context, d transport.Delivery) error {
		msg := d.Message()
		if err := service.Handle(ctx, msg.Payload, msg.Metadata["event_type"]); err != nil {
			return err
		}
		if err := d.Ack(); err != nil {
			return err
		}
		select {
		case received <- msg:
			return nil
		default:
			return errors.New("fixture receipt queue full")
		}
	}, func(context.Context, transport.Failure) error {
		return errors.New("fixture unexpected failed delivery")
	}))
	_, err = pool.ExecContext(ctx, `RENAME TABLE iam_fresh_bootstrap_journal TO isolated_bootstrap_journal,
		iam_bootstrap_policy_notification TO isolated_bootstrap_notifications`)
	require.NoError(t, err)
	claimMissing := newOwner()
	require.ErrorContains(t, claimMissing.Start(ctx), "no original claim")
	require.Zero(t, count("SELECT COUNT(*) FROM rm_tx_request"))
	_, err = pool.ExecContext(ctx, `RENAME TABLE isolated_bootstrap_journal TO iam_fresh_bootstrap_journal,
		isolated_bootstrap_notifications TO iam_bootstrap_policy_notification`)
	require.NoError(t, err)
	plans := map[int]transaction.FrozenPlan{}
	for _, phase := range []int{33, 35} {
		var inputBytes []byte
		require.NoError(t, pool.QueryRowContext(ctx, "SELECT input_bytes FROM iam_bootstrap_policy_notification WHERE phase=?", phase).Scan(&inputBytes))
		var in message.Input
		require.NoError(t, json.Unmarshal(inputBytes, &in))
		msg, err := message.New(in)
		require.NoError(t, err)
		policy, err := eventing.PolicyPayloadVersion(in.Payload)
		require.NoError(t, err)
		wire, err := legacy.Encode(legacy.Envelope{UUID: in.ID, Payload: in.Payload, Metadata: eventing.PolicyVersionWireMetadata(policy)}, legacy.Revision1)
		require.NoError(t, err)
		plans[phase], err = transaction.NewPlan(transaction.PlanInput{
			Owner: binding.Owner, Scope: in.Scope, OperationID: "bootstrap-delivery:" + in.ID, IdempotencyKey: "bootstrap-delivery:" + in.ID, InputHash: sha256.Sum256(inputBytes),
			Branches: []transaction.BranchInput{{ID: "policy", PhysicalTopic: physical, RouteRevision: "m3-1", Message: msg, Wire: wire}},
		})
		require.NoError(t, err)
	}
	_, err = pool.ExecContext(ctx, `INSERT INTO domain_event_outbox
		(event_id,event_type,aggregate_type,aggregate_id,topic_name,payload_json,status,next_attempt_at,created_at,updated_at)
		VALUES('old-pending','fixture','fixture','fixture','fixture','{}','pending',NOW(),NOW(),NOW())`)
	require.NoError(t, err)
	blocked := newOwner()
	require.ErrorContains(t, blocked.Start(ctx), "old authorization publication responsibilities must be drained")
	require.Zero(t, count("SELECT COUNT(*) FROM rm_tx_request"))
	_, err = pool.ExecContext(ctx, `UPDATE domain_event_outbox SET status='published' WHERE event_id='old-pending'`)
	require.NoError(t, err)
	// The interrupted process reserved the original request and died before
	// BeginPrepare. Its replacement must keep Checker live while it waits.
	first := plans[33]
	reserved, err := store.Reserve(ctx, transaction.RequestFor(first), first, transaction.ReserveOptions{Lease: 10 * time.Second})
	require.NoError(t, err)
	require.True(t, reserved.Acquired)
	old := reserved.Execution.References()[0]
	paused := newOwner()
	require.NoError(t, paused.Start(ctx))
	require.Error(t, paused.CheckReady(ctx))
	require.Equal(t, transaction.Unknown, (transaction.StoreChecker{Store: store}).Check(ctx, old))
	require.Zero(t, count("SELECT COUNT(*) FROM rm_tx_execution WHERE prepare_started=TRUE"))
	require.NoError(t, paused.Stop(ctx))
	// Keep the second delivery inside its original SQL root while stopping.
	held, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer held.Rollback()
	var lockedPhase int
	require.NoError(t, held.QueryRowContext(ctx, "SELECT phase FROM iam_bootstrap_policy_notification WHERE phase=35 FOR UPDATE").Scan(&lockedPhase))
	require.Equal(t, 35, lockedPhase)
	owner := newOwner()
	require.NoError(t, owner.Start(ctx))
	for _, phase := range []int{33, 35} {
		plan := plans[phase]
		branch := plan.Input().Branches[0]
		if phase == 35 {
			key := transaction.RequestFor(plan).KeyDigest()
			require.Eventually(t, func() bool {
				var prepared int
				err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM rm_tx_witness w JOIN rm_tx_request r ON r.active_execution=w.execution_key
					WHERE r.request_key=? AND w.state='prepared' AND w.half_outcome='prepared'`, key[:]).Scan(&prepared)
				return err == nil && prepared == 1
			}, 7*time.Second, 50*time.Millisecond)
			deadline, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
			err := owner.Stop(deadline)
			cancel()
			require.ErrorIs(t, err, transaction.ErrShutdownIncomplete)
			require.Equal(t, 1, count("SELECT COUNT(*) FROM iam_bootstrap_policy_notification WHERE delivery_committed=1"))
			require.NoError(t, held.Rollback())
		}
		select {
		case got := <-received:
			in := branch.Message.Input()
			require.Equal(t, in.ID, got.ID)
			require.Equal(t, in.Payload, got.Payload)
			require.Equal(t, branch.Wire, got.Wire)
		case <-ctx.Done():
			t.Fatalf("fixture timed out awaiting phase %d delivery", phase)
		}
		current, err := store.Lookup(ctx, transaction.RequestFor(plan))
		require.NoError(t, err)
		require.Equal(t, transaction.Committed, current.State)
		require.Equal(t, plan.Digest(), current.Execution.Plan.Digest())
		require.Equal(t, transaction.Commit, (transaction.StoreChecker{Store: store}).Check(ctx, current.Execution.References()[0]))
		if phase == 33 {
			require.EqualValues(t, 2, current.Execution.Fence)
		}
		t.Logf("BROKER_PROOF phase=%d fence=%d original_wire=true ack=true", phase, current.Execution.Fence)
	}
	require.True(t, consumerRuntime.PolicyVersionLoaded(4))
	require.Equal(t, transaction.Rollback, (transaction.StoreChecker{Store: store}).Check(ctx, old))
	require.NoError(t, owner.Stop(ctx))
	owner = newOwner()
	require.NoError(t, owner.Start(ctx))
	require.Eventually(t, func() bool { return owner.CheckReady(ctx) == nil }, 4*time.Second, 50*time.Millisecond)
	require.NoError(t, owner.Stop(ctx))
	require.Equal(t, 2, count("SELECT COUNT(*) FROM iam_bootstrap_policy_notification WHERE delivery_committed=1"))
	require.Equal(t, 2, count("SELECT COUNT(*) FROM rm_tx_request"))
	require.Equal(t, 3, count("SELECT COUNT(*) FROM rm_tx_execution"))
	require.Equal(t, 3, count("SELECT COUNT(*) FROM rm_tx_witness"))
	require.Equal(t, 2, count("SELECT COUNT(*) FROM rm_tx_witness WHERE final_outcome='commit_confirmed'"))
}
