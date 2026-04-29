package container

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitEventingKeepsOutboxPendingWhenEventBusUnavailable(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("events.catalog_path", filepath.Join(testRepoRoot(t), "configs", "events.yaml"))

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	c := NewContainer(db, nil, nil, nil)

	require.NoError(t, c.initEventing())
	require.NotNil(t, c.outboxStore)
	require.Nil(t, c.outboxRelay)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
