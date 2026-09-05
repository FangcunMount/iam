package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstanceChannelIsEphemeralAndInstanceScoped(t *testing.T) {
	t.Parallel()

	first := InstanceChannel("iam-host-1", 1001)
	second := InstanceChannel("iam-host-2", 1002)

	require.Equal(t, "iam-policy-sync.iam-host-1.1001#ephemeral", first)
	require.Equal(t, "iam-policy-sync.iam-host-2.1002#ephemeral", second)
	require.NotEqual(t, first, second)
}

func TestInstanceChannelSanitizesHostName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "iam-policy-sync.iam-host.1001#ephemeral", InstanceChannel(" iam/host ", 1001))
}
