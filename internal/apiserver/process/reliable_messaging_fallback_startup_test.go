//go:build reliable_messaging

package process

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/FangcunMount/component-base/pkg/processruntime"
	"github.com/FangcunMount/iam/v5/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

// This covers the old application's full bootstrap composition on the new
// schema. The isolated database is seeded and drained before SDK ownership.
// Socket listeners, the production image and the live rollback sequence are
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
	opts.SecureServing.BindPort = 0

	// Fresh bootstrap can stage its own legacy role events. They are fixture
	// data and must be drained before the SDK becomes the only writer.
	cfg, err := config.CreateConfigFromOptions(opts)
	require.NoError(t, err)
	migrator := NewDatabaseManager(cfg)
	require.NoError(t, migrator.Initialize())
	require.NoError(t, migrator.Close())
	_, err = admin.Exec("UPDATE " + database + ".domain_event_outbox SET status='published' WHERE status <> 'published'")
	require.NoError(t, err)

	opts.Events.ReliableMessaging.Enabled = true
	opts.NSQOptions.Enabled = true
	opts.NSQOptions.NSQdAddr = os.Getenv("RM_IAM_NSQ_TCP")
	require.NotEmpty(t, opts.NSQOptions.NSQdAddr)
	server, err := createAPIServer(cfg)
	require.NoError(t, err)
	prepared, err := server.PrepareRun()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, runShutdownSequence(server.buildShutdownSequenceDeps(processruntime.Lifecycle{})))
	})
	require.NotNil(t, prepared.genericAPIServer)
	require.NotNil(t, prepared.grpcServer)
	require.NotNil(t, prepared.container.BuildRuntimeDeps().ReliableMessaging)

	response := httptest.NewRecorder()
	prepared.genericAPIServer.Engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}
