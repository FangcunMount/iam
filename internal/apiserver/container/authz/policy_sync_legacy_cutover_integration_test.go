//go:build reliable_messaging

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/messaging"
	oldnsq "github.com/FangcunMount/component-base/pkg/messaging/nsq"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/nsqio/go-nsq"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// The current IAM old image uses component-base v0.6.1. Unlike v0.6.11 it
// cannot hand a failed message to a stable failure channel. A new IAM instance
// must recover the authorization state from its version source instead.
// This test exercises the released old Subscriber and the IAM SDK wiring in
// disposable MySQL and NSQ. A separate SDK process also verifies cold-start
// recovery after the old ephemeral channel disappears. Neither is production.
func TestIAMV061CutoffThenSDKReconcilesMySQLVersion(t *testing.T) {
	if os.Getenv("RM_IAM_M6_REQUIRED") != "1" {
		t.Skip("disposable NSQ required")
	}
	tcp, httpURL := os.Getenv("RM_IAM_NSQ_TCP"), os.Getenv("RM_IAM_NSQ_HTTP")
	require.NotEmpty(t, tcp)
	require.NotEmpty(t, httpURL)
	require.NotEmpty(t, os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"), "disposable MySQL admin DSN required")
	db := authzdb.Open(t, true)
	require.NoError(t, db.Exec("INSERT INTO users(id,status) VALUES(2,1)").Error)
	uow := authzuow.NewUnitOfWork(db, nil, authzdb.Stager(t, db))
	probeRole, err := role.NewRole("qs:m6-v061-probe", "isolated cutover probe")
	require.NoError(t, err)
	probeResource, err := resource.NewResource("qs:evaluation:collection:assessments", []string{"retry"}, resource.WithDisplayName("Assessments"))
	require.NoError(t, err)
	var grant permissiongrant.Grant
	require.NoError(t, uow.WithinTx(context.Background(), func(txctx context.Context, repos appuow.TxRepositories) error {
		if e := repos.Roles.Create(txctx, &probeRole); e != nil {
			return e
		}
		if e := repos.Resources.Create(txctx, &probeResource); e != nil {
			return e
		}
		var e error
		grant, e = permissiongrant.New(probeRole.ID, probeResource.ID, probeResource.KeyString(), "retry", "isolated-cutover")
		if e != nil {
			return e
		}
		if e = repos.PermissionGrants.Create(txctx, &grant); e != nil {
			return e
		}
		assigned, e := assignment.NewAssignment(assignment.SubjectType("user"), meta.FromUint64(2), probeRole.ID, assignment.WithGrantedBy("isolated-cutover"))
		if e != nil {
			return e
		}
		return repos.Assignments.Create(txctx, &assigned)
	}))
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, probeResource.KeyString(), "retry")
	require.NoError(t, err)
	runtime, err := authzruntime.NewRuntime(context.Background(), authzruntime.NewMySQLSource(db), authorization.NewEvaluator(),
		authzruntime.WithConfig(authzruntime.Config{CheckInterval: 20 * time.Millisecond, SyncTimeout: time.Second, MaxUnconfirmed: 10 * time.Second}))
	require.NoError(t, err)
	require.True(t, runtime.PolicyVersionLoaded(1))
	before, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.True(t, before.Allowed)
	runtime.RequireSync()
	host, portText, err := net.SplitHostPort(tcp)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	lookup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("topic") != policypublication.Topic {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-NSQ-Content-Type", "nsq; version=1.0")
		_ = json.NewEncoder(w).Encode(map[string]any{"producers": []map[string]any{{"broadcast_address": host, "tcp_port": port}}})
	}))
	defer lookup.Close()

	oldChannel := InstanceChannel("v061-cutover", int(time.Now().UnixNano()%1000000000))
	config := nsq.NewConfig()
	config.MaxAttempts = 1 // shorten the released driver's cutoff, without changing its behavior
	config.MaxInFlight = 1
	config.DefaultRequeueDelay = 20 * time.Millisecond
	config.MaxRequeueDelay = 20 * time.Millisecond
	old, err := oldnsq.NewSubscriber([]string{lookup.URL}, config)
	require.NoError(t, err)
	defer old.Close()
	var oldCalls atomic.Int32
	require.NoError(t, old.Subscribe(policypublication.Topic, oldChannel, func(context.Context, *messaging.Message) error {
		oldCalls.Add(1)
		return errors.New("controlled old policy reload failure")
	}))
	require.Eventually(t, func() bool {
		return nsqChannelClients(httpURL, policypublication.Topic, oldChannel) > 0
	}, 10*time.Second, 20*time.Millisecond, "old channel must be attached before publish")
	require.NoError(t, uow.WithinTx(context.Background(), func(txctx context.Context, repos appuow.TxRepositories) error {
		if _, e := repos.PermissionGrants.AtomicRevoke(txctx, grant.ID); e != nil {
			return e
		}
		_, e := repos.PolicyVersions.Increment(txctx, "isolated-cutover", "old-notification-failed")
		return e
	}))
	committedVersion, err := authzruntime.NewMySQLSource(db).ReadVersion(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, committedVersion)
	require.True(t, runtime.PolicyVersionLoaded(1), "new runtime has not received the committed change yet")
	publisher, err := oldnsq.NewPublisher(tcp, nil)
	require.NoError(t, err)
	defer publisher.Close()
	message := messaging.NewMessage("iam-v061-cutover-event", []byte(`{"version":2}`))
	message.Metadata["event_type"] = "iam.authz.version_changed.v2"
	require.NoError(t, publisher.PublishMessage(context.Background(), policypublication.Topic, message))
	require.Eventually(t, func() bool {
		provider, ok := old.(interface{ Stats() map[string]interface{} })
		if !ok {
			return false
		}
		stats, ok := provider.Stats()["consumer_0"].(map[string]interface{})
		if !ok {
			return false
		}
		finished, _ := stats["finished"].(uint64)
		requeued, _ := stats["requeued"].(uint64)
		return finished == 1 && requeued >= 1
	}, 10*time.Second, 20*time.Millisecond, "old driver should FIN after its bounded retry")
	require.EqualValues(t, 1, oldCalls.Load(), "second delivery must skip the business handler")
	require.NoError(t, old.Close())
	require.Eventually(t, func() bool {
		return nsqChannelClients(httpURL, policypublication.Topic, oldChannel) == -1
	}, 10*time.Second, 20*time.Millisecond, "old ephemeral channel should disappear")
	var dbName string
	require.NoError(t, db.Raw("SELECT DATABASE()").Scan(&dbName).Error)
	require.NotEmpty(t, dbName)
	childDSN, err := mysqldriver.ParseDSN(os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN"))
	require.NoError(t, err)
	childDSN.DBName = dbName
	childDSN.ParseTime = true
	childCtx, cancelChild := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelChild()
	child := exec.CommandContext(childCtx, os.Args[0], "-test.run=^TestIAMV061SDKColdStartProcess$", "-test.v")
	child.Env = append(os.Environ(),
		"RM_IAM_CUTOVER_CHILD=1",
		"RM_IAM_CUTOVER_DSN="+childDSN.FormatDSN(),
		"RM_IAM_CUTOVER_RESOURCE="+probeResource.KeyString(),
	)
	output, err := child.CombinedOutput()
	require.NoErrorf(t, err, "independent SDK cold start failed: %s", output)
	require.NoError(t, childCtx.Err())
	require.True(t, runtime.PolicyVersionLoaded(1), "the parent has not reconciled during the child process")

	driverConfig := nsq.NewConfig()
	driverConfig.DialTimeout = time.Second
	sdkSubscriber, err := sdknsq.NewSubscriber(sdknsq.SubscriberConfig{
		NSQDAddresses: []string{tcp}, Driver: driverConfig, MaxInFlight: 1, MaxAttempts: 1,
		FailedHandoffGroup: ChannelPrefix,
	})
	require.NoError(t, err)
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 5 * time.Second}, []string{httpURL})
	require.NoError(t, err)
	var newFailures atomic.Int32
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	syncer := module.SDKPolicySyncSubscriber(sdkSubscriber, provisioner, func(context.Context, legacy.FailedHandoff) error {
		newFailures.Add(1)
		return nil
	})
	require.NotNil(t, syncer)
	syncer.channel = InstanceChannel("sdk-cutover", int(time.Now().UnixNano()%1000000000))
	require.NoError(t, syncer.Start(context.Background()))
	defer syncer.Stop()
	require.True(t, syncer.registered)
	require.True(t, runtime.PolicyVersionLoaded(2), "startup reconciliation must recover the committed MySQL version")
	after, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, after.Allowed, "the committed revocation must be effective without the old notification")
	require.EqualValues(t, 2, after.PolicyVersion)
	require.Zero(t, newFailures.Load(), "no old stable handoff exists to audit")
}

func TestIAMV061SDKColdStartProcess(t *testing.T) {
	if os.Getenv("RM_IAM_CUTOVER_CHILD") != "1" {
		t.Skip("only run by the cutover parent process")
	}
	assertRequired := func(name string) string {
		value := os.Getenv(name)
		require.NotEmpty(t, value)
		return value
	}
	db, err := gorm.Open(gormmysql.Open(assertRequired("RM_IAM_CUTOVER_DSN")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	runtime, err := authzruntime.NewRuntime(context.Background(), authzruntime.NewMySQLSource(db), authorization.NewEvaluator(),
		authzruntime.WithConfig(authzruntime.Config{CheckInterval: 20 * time.Millisecond, SyncTimeout: time.Second, MaxUnconfirmed: 10 * time.Second}))
	require.NoError(t, err)
	require.True(t, runtime.PolicyVersionLoaded(2))
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, assertRequired("RM_IAM_CUTOVER_RESOURCE"), "retry")
	require.NoError(t, err)
	driverConfig := nsq.NewConfig()
	driverConfig.DialTimeout = time.Second
	subscriber, err := sdknsq.NewSubscriber(sdknsq.SubscriberConfig{
		NSQDAddresses: []string{assertRequired("RM_IAM_NSQ_TCP")}, Driver: driverConfig, MaxInFlight: 1, MaxAttempts: 1,
		FailedHandoffGroup: ChannelPrefix,
	})
	require.NoError(t, err)
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 5 * time.Second}, []string{assertRequired("RM_IAM_NSQ_HTTP")})
	require.NoError(t, err)
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	syncer := module.SDKPolicySyncSubscriber(subscriber, provisioner, func(context.Context, legacy.FailedHandoff) error {
		return errors.New("unexpected failed handoff from old v0.6.1 subscriber")
	})
	require.NotNil(t, syncer)
	syncer.channel = InstanceChannel("sdk-cold-start", os.Getpid())
	require.NoError(t, syncer.Start(context.Background()))
	defer syncer.Stop()
	require.True(t, syncer.registered)
	decision, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.EqualValues(t, 2, decision.PolicyVersion)
}

func nsqChannelClients(base, topic, channel string) int {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/stats?format=json")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var state struct {
		Topics []struct {
			Name     string `json:"topic_name"`
			Channels []struct {
				Name    string            `json:"channel_name"`
				Clients []json.RawMessage `json:"clients"`
			} `json:"channels"`
		} `json:"topics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return 0
	}
	for _, candidate := range state.Topics {
		if candidate.Name == topic {
			for _, current := range candidate.Channels {
				if current.Name == channel {
					return len(current.Clients)
				}
			}
		}
	}
	return -1
}
