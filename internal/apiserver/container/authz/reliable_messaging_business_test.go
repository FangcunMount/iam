//go:build reliable_messaging

package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	cbnsq "github.com/FangcunMount/component-base/pkg/messaging/nsq"
	authzapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/authzdb"
	authzgrpc "github.com/FangcunMount/iam/v5/internal/apiserver/transport/grpc/service/authz"
	servergrpc "github.com/FangcunMount/iam/v5/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/testutil/tlsfixture"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	"github.com/stretchr/testify/require"
)

func TestReliableMessagingIAMQSBusinessRoundtrip(t *testing.T) {
	require.Equal(t, "1", os.Getenv("RM_BUSINESS_REQUIRED"))
	for _, key := range []string{"IAM_AUTHZ_TEST_MYSQL_DSN", "RM_IAM_EVENTS_CATALOG", "RM_IAM_NSQ_TCP", "RM_IAM_NSQ_HTTP", "RM_NSQ_LOOKUP", "RM_QS_PROOF"} {
		require.NotEmpty(t, os.Getenv(key), key)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	db := authzdb.Open(t, true)
	require.NoError(t, db.Exec(sdkmysql.Schema).Error)
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,dirty BOOLEAN NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations VALUES(38,FALSE)").Error)
	require.NoError(t, db.Exec("INSERT INTO users(id,status) VALUES(2,1)").Error)
	cfg := cbmessaging.DefaultConfig()
	cfg.NSQ.NSQdAddr = os.Getenv("RM_IAM_NSQ_TCP")
	cfg.NSQ.LookupdAddrs = []string{os.Getenv("RM_NSQ_LOOKUP")}
	bus, err := cbmessaging.NewEventBus(cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, bus.Close()) }()
	const topic = "iam.authz.version.v2"
	require.NoError(t, cbnsq.NewTopicCreator(cfg.NSQ.NSQdAddr, slog.Default()).EnsureTopics([]string{topic}))
	httpClient := &http.Client{Timeout: time.Second}
	get := func(url string, value any) bool {
		response, e := httpClient.Get(url)
		if e != nil {
			return false
		}
		defer func() { _ = response.Body.Close() }()
		return response.StatusCode == http.StatusOK && json.NewDecoder(response.Body).Decode(value) == nil
	}
	require.Eventually(t, func() bool {
		var lookup struct {
			Producers []json.RawMessage `json:"producers"`
		}
		return get("http://"+os.Getenv("RM_NSQ_LOOKUP")+"/lookup?topic="+topic, &lookup) && len(lookup.Producers) > 0
	}, 5*time.Second, 20*time.Millisecond)
	opts := options.DefaultReliableMessagingOptions()
	opts.Enabled = true
	owner, err := platform.InitEventing(platform.EventingDeps{DB: db, EventBus: bus, CatalogPath: os.Getenv("RM_IAM_EVENTS_CATALOG"), NSQEnabled: true, NSQAddress: cfg.NSQ.NSQdAddr, ReliableMessaging: opts, OutboxInterval: 10 * time.Millisecond})
	require.NoError(t, err)
	defer func() {
		stopCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, owner.ReliableRuntime.Stop(stopCtx))
		owner.CloseReliableProducer()
	}()
	uow := authzuow.NewUnitOfWork(db, nil, owner.Stager)
	probeRole, err := role.NewRole("qs:m3-probe", "isolated MQ probe")
	require.NoError(t, err)
	probeResource, err := resource.NewResource("qs:assessment:collection:probe", []string{"read"}, resource.WithDisplayName("Isolated MQ permission probe"))
	require.NoError(t, err)
	var grant permissiongrant.Grant
	require.NoError(t, uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if e := repos.Roles.Create(txctx, &probeRole); e != nil {
			return e
		}
		if e := repos.Resources.Create(txctx, &probeResource); e != nil {
			return e
		}
		var e error
		grant, e = permissiongrant.New(probeRole.ID, probeResource.ID, probeResource.KeyString(), "read", "isolated-proof")
		if e != nil {
			return e
		}
		if e = repos.PermissionGrants.Create(txctx, &grant); e != nil {
			return e
		}
		assigned, e := assignment.NewAssignment(assignment.SubjectType("user"), meta.FromUint64(2), probeRole.ID, assignment.WithGrantedBy("isolated-proof"))
		if e != nil {
			return e
		}
		return repos.Assignments.Create(txctx, &assigned)
	}))
	runtime, err := authzruntime.NewRuntime(ctx, authzruntime.NewMySQLSource(db), authorization.NewEvaluator())
	require.NoError(t, err)
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	syncer := module.PolicySyncSubscriber(bus.Subscriber())
	require.NoError(t, syncer.Start(ctx))
	defer func() { require.NoError(t, syncer.Stop()) }()
	// Ephemeral certificates exercise the actual mTLS/ACL stack; they make no
	// claim about production certificate validity or deployment configuration.
	ca := tlsfixture.New(t)
	serverPair := ca.Issue(t, "localhost", false)
	clientPair := ca.Issue(t, "qs-apiserver.svc", false)
	grpcConfig := servergrpc.NewConfig()
	grpcConfig.Insecure = false
	grpcConfig.TLSCertFile, grpcConfig.TLSKeyFile = serverPair.CertFile, serverPair.KeyFile
	grpcConfig.MTLS.Enabled, grpcConfig.MTLS.RequireClientCert = true, true
	grpcConfig.MTLS.EnableAutoReload = false
	grpcConfig.MTLS.CAFile = ca.CAFile
	grpcConfig.ACL.Enabled = true
	grpcConfig.ACL.ConfigFile = os.Getenv("RM_IAM_GRPC_ACL")
	require.NotEmpty(t, grpcConfig.ACL.ConfigFile)
	server, err := servergrpc.NewServer(grpcConfig)
	require.NoError(t, err)
	grpcServer := server.Server
	authzgrpc.NewService(authzapp.NewDecisionService(runtime), authzapp.NewSnapshotReader(runtime), nil, nil).Register(grpcServer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer grpcServer.Stop()
	go func() { _ = grpcServer.Serve(listener) }()
	require.NoError(t, owner.ReliableRuntime.Start(ctx))
	var mu sync.Mutex
	phase := 0
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodPost || (phase == 0 && r.URL.Path != "/revoke") || (phase == 1 && r.URL.Path != "/restore") || phase > 1 {
			http.Error(w, "unexpected phase", 409)
			return
		}
		// Confirm both distinct real subscriptions are present before the single
		// mutation; no extra notification is used to hide a missing consumer.
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			var stats struct {
				Topics []struct {
					TopicName string `json:"topic_name"`
					Channels  []struct {
						ChannelName string            `json:"channel_name"`
						Clients     []json.RawMessage `json:"clients"`
					} `json:"channels"`
				} `json:"topics"`
			}
			if get(os.Getenv("RM_IAM_NSQ_HTTP")+"/stats?format=json", &stats) {
				live := 0
				for _, item := range stats.Topics {
					if item.TopicName == topic {
						for _, channel := range item.Channels {
							if len(channel.Clients) > 0 {
								live++
							}
						}
					}
				}
				if live == 2 {
					ready = true
					break
				}
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
		if !ready {
			http.Error(w, "two live subscriptions required", 503)
			return
		}
		err := uow.WithinTx(r.Context(), func(txctx context.Context, repos appuow.TxRepositories) error {
			if phase == 0 {
				if _, e := repos.PermissionGrants.AtomicRevoke(txctx, grant.ID); e != nil {
					return e
				}
			} else {
				restored, e := permissiongrant.New(probeRole.ID, probeResource.ID, probeResource.KeyString(), "read", "isolated-restore")
				if e != nil {
					return e
				}
				if e = repos.PermissionGrants.Create(txctx, &restored); e != nil {
					return e
				}
			}
			version, e := repos.PolicyVersions.Increment(txctx, "isolated-proof", r.URL.Path)
			if e != nil {
				return e
			}
			return repos.Events.Stage(txctx, policy.NewVersionChangedEvent(version.Version))
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("fixture mutation failed: %v", err), 500)
			return
		}
		phase++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer control.Close()
	cmd := exec.CommandContext(ctx, os.Getenv("RM_QS_PROOF"), "-test.run=^TestReliableMessagingIAMPolicyRoundtrip$", "-test.v")
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cmd.Env = append(os.Environ(), "RM_IAM_GRPC=localhost:"+port, "RM_IAM_CONTROL="+control.URL,
		"RM_TLS_CA="+ca.CAFile, "RM_TLS_CERT="+clientPair.CertFile, "RM_TLS_KEY="+clientPair.KeyFile)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
	mu.Lock()
	done := phase
	mu.Unlock()
	require.Equal(t, 2, done)
	require.EqualValues(t, 3, runtime.RuntimeHealthDetails()["last_event_version"], "IAM must receive the event, not just reload by polling")
	var total, published int64
	require.NoError(t, db.Table("rm_outbox").Count(&total).Error)
	require.NoError(t, db.Table("rm_outbox").Where("state='published'").Count(&published).Error)
	require.EqualValues(t, 2, total)
	require.Equal(t, total, published)
	var legacy int64
	require.NoError(t, db.Table("domain_event_outbox").Count(&legacy).Error)
	require.Zero(t, legacy)
}
