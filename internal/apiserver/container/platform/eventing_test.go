package platform

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/options"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A deliberately unusable DB and catalog prove rejection precedes both.
func TestInitEventingRejectsRetiredAssemblyBeforeDependencies(t *testing.T) {
	for _, provider := range []string{"", "nsq", "rocketmq"} {
		for _, relay := range []bool{false, true} {
			t.Run(provider+fmt.Sprint(relay), func(t *testing.T) {
				opts := options.DefaultReliableMessagingOptions()
				opts.Enabled = relay
				result, err := InitEventing(EventingDeps{DB: &gorm.DB{}, CatalogPath: "does-not-exist", Messaging: options.MessagingOptions{Provider: provider}, ReliableMessaging: opts})
				require.ErrorContains(t, err, "final RocketMQ transaction owner required")
				require.ErrorIs(t, err, eventoutbox.ErrUnsafeMessagingHandoff)
				require.Nil(t, result)
			})
		}
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
