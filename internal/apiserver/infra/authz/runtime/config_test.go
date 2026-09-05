package runtime_test

import (
	"testing"
	"time"

	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	"github.com/stretchr/testify/require"
)

func TestPolicySyncConfigBudgets(t *testing.T) {
	require.NoError(t, authzruntime.DefaultConfig().Validate())
	for _, config := range []authzruntime.Config{{}, {CheckInterval: time.Second, SyncTimeout: time.Second, MaxUnconfirmed: time.Second}, {CheckInterval: -time.Second, SyncTimeout: time.Second, MaxUnconfirmed: time.Minute}, {CheckInterval: time.Second, SyncTimeout: time.Minute, MaxUnconfirmed: time.Minute}} {
		require.Error(t, config.Validate())
	}
}
