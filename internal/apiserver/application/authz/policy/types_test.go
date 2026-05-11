package policy

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPermissionCommandUsesValueObjects(t *testing.T) {
	t.Parallel()

	cmd, err := NewAddPermissionCommand(meta.FromUint64(10), resource.NewResourceID(20), " read ", scope.Default(), " tenant-a ", " operator-1 ", "grant")

	require.NoError(t, err)
	require.Equal(t, "read", cmd.ActionString())
	require.Equal(t, "tenant-a", cmd.TenantIDString())
	require.Equal(t, "operator-1", cmd.ChangedByString())
	require.Equal(t, "grant", cmd.Reason)
}

func TestPermissionCommandRejectsActionPatterns(t *testing.T) {
	t.Parallel()

	_, err := NewAddPermissionCommand(meta.FromUint64(10), resource.NewResourceID(20), "read|list", scope.Default(), "tenant-a", "operator-1", "")
	require.Error(t, err)

	_, err = NewRemovePermissionCommand(meta.FromUint64(10), resource.NewResourceID(20), ".*", scope.Default(), "tenant-a", "operator-1", "")
	require.Error(t, err)
}
