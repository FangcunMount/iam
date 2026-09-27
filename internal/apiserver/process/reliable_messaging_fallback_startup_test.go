//go:build reliable_messaging

package process

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/iam/v5/internal/apiserver/config"
	mysqljwks "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/jwks"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/token/keyset"
	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	pkgauth "github.com/FangcunMount/iam/v5/pkg/auth"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// This covers the old application's full bootstrap composition on the new
// schema. The isolated database is seeded and drained before SDK ownership.
// The production image, certificates and live rollback sequence remain
// separate release gates.
func TestFallbackSchema39APICompositionReadiness(t *testing.T) {
	if os.Getenv("RM_IAM_PROCESS_MYSQL") == "" {
		t.Skip("isolated MySQL fixture required")
	}
	const database = "iam_fallback_process"
	admin, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/?parseTime=true&loc=UTC")
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	_, err = admin.Exec("CREATE DATABASE " + database)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + database) })

	redis := miniredis.RunT(t)
	opts := apiserveroptions.NewOptions()
	opts.MySQLOptions.Host = "127.0.0.1:3306"
	opts.MySQLOptions.Username = "root"
	opts.MySQLOptions.Database = database
	opts.MigrationOptions.Database = database
	opts.Events.CatalogPath = os.Getenv("RM_IAM_EVENTS_CATALOG")
	require.NotEmpty(t, opts.Events.CatalogPath)
	opts.RedisOptions.Cache.Host, opts.RedisOptions.Cache.Port = splitRedisAddr(t, redis.Addr())
	opts.IDP.EncryptionKey = "0123456789abcdef0123456789abcdef"
	opts.JWKS.KeysDir = t.TempDir()
	opts.GRPCOptions.AuthzAssignmentConstraintsFile = os.Getenv("RM_IAM_ASSIGNMENT_CONSTRAINTS")
	require.NotEmpty(t, opts.GRPCOptions.AuthzAssignmentConstraintsFile)
	opts.InsecureServing.BindPort = 19080
	opts.SecureServing.BindPort = 0
	opts.GRPCOptions.BindPort = 19090
	opts.GRPCOptions.HealthzPort = 19091

	// Fresh bootstrap can stage its own legacy role events. They are fixture
	// data and must be drained before the SDK becomes the only writer.
	cfg, err := config.CreateConfigFromOptions(opts)
	require.NoError(t, err)
	migrator := NewDatabaseManager(cfg)
	require.NoError(t, migrator.Initialize())
	require.NoError(t, migrator.Close())
	_, err = admin.Exec("UPDATE " + database + ".domain_event_outbox SET status='published' WHERE status <> 'published'")
	require.NoError(t, err)
	// Production rollback reuses an existing signing key. Seed one that was
	// already valid before this startup, avoiding DATETIME(0) rounding of a
	// freshly auto-created key's NotBefore into the next second.
	keyDB, err := gorm.Open(gormmysql.Open("root@tcp(127.0.0.1:3306)/"+database+"?parseTime=true&loc=UTC"), &gorm.Config{})
	require.NoError(t, err)
	keySQL, err := keyDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = keySQL.Close() })
	keys := keyset.NewKeyManagerWithPolicy(
		mysqljwks.NewKeyRepository(keyDB),
		keyset.NewRSAKeyGenerator(),
		keyset.NewPEMPrivateKeyStorage(opts.JWKS.KeysDir),
		keyset.DefaultRotationPolicy(),
	)
	notBefore := time.Now().Add(-2 * time.Second)
	_, err = keys.CreateKey(context.Background(), pkgauth.TokenProfileAlgorithm, &notBefore, nil)
	require.NoError(t, err)

	opts.Events.ReliableMessaging.Enabled = true
	opts.NSQOptions.Enabled = true
	opts.NSQOptions.NSQdAddr = os.Getenv("RM_IAM_NSQ_TCP")
	require.NotEmpty(t, opts.NSQOptions.NSQdAddr)
	server, err := createAPIServer(cfg)
	require.NoError(t, err)
	prepared, err := server.PrepareRun()
	require.NoError(t, err)
	runDone := make(chan error, 1)
	go func() { runDone <- prepared.Run() }()
	t.Cleanup(func() {
		require.NoError(t, runShutdownSequence(server.buildShutdownSequenceDeps(processruntime.Lifecycle{})))
		select {
		case <-runDone:
		case <-time.After(10 * time.Second):
			t.Error("fallback process listeners did not stop")
		}
	})
	require.NotNil(t, prepared.genericAPIServer)
	require.NotNil(t, prepared.grpcServer)
	require.NotNil(t, prepared.container.BuildRuntimeDeps().ReliableMessaging)

	response := httptest.NewRecorder()
	prepared.genericAPIServer.Engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	client := &http.Client{Timeout: time.Second}
	for _, address := range []string{"http://127.0.0.1:19080/readyz", "http://127.0.0.1:19091/readyz"} {
		require.Eventually(t, func() bool {
			response, err := client.Get(address)
			if err != nil {
				return false
			}
			_ = response.Body.Close()
			return response.StatusCode == http.StatusOK
		}, 12*time.Second, 100*time.Millisecond, address)
	}
	select {
	case err := <-runDone:
		t.Fatalf("fallback process exited while ready: %v", err)
	default:
	}
}
