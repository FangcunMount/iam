package platform

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitEventingKeepsOutboxPendingWhenEventBusUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	eventing, err := InitEventing(EventingDeps{
		DB:          db,
		CatalogPath: filepath.Join(testRepoRoot(t), "configs", "events.yaml"),
	})
	require.NoError(t, err)
	require.NotNil(t, eventing.Outbox)
	require.Nil(t, eventing.Relay)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
