package platform

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/options"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitEventingRejectsRetiredLegacyRuntime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	eventing, err := InitEventing(EventingDeps{
		DB:          db,
		CatalogPath: filepath.Join(testRepoRoot(t), "configs", "events.yaml"),
	})
	require.ErrorContains(t, err, "legacy Outbox runtime is retired")
	require.Nil(t, eventing)

}

func TestInitReliableEventingFailsWithoutDependencies(t *testing.T) {
	opts := options.DefaultReliableMessagingOptions()
	opts.Enabled = true
	result, err := InitEventing(EventingDeps{ReliableMessaging: opts, CatalogPath: filepath.Join(testRepoRoot(t), "configs", "events.yaml")})
	require.ErrorContains(t, err, "requires IAM MySQL")
	require.Nil(t, result, "explicit reliable mode must not return a legacy fallback")
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
