package container

import (
	"path/filepath"
	"runtime"
	"testing"

	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitEventingKeepsOutboxPendingWhenEventBusUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	c := NewContainerWithOptions(db, nil, nil, nil, RuntimeOptions{
		Events: apiserveroptions.EventOptions{
			CatalogPath: filepath.Join(testRepoRoot(t), "configs", "events.yaml"),
		},
	})

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
