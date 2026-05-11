package role

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCreateRoleCommandUsesValueObjects(t *testing.T) {
	t.Parallel()

	cmd, err := NewCreateRoleCommand(" iam:admin ", " Admin ", " tenant-a ", "desc")

	require.NoError(t, err)
	require.Equal(t, "iam:admin", cmd.NameString())
	require.Equal(t, "Admin", cmd.DisplayName)
	require.Equal(t, "tenant-a", cmd.TenantIDString())
	require.Equal(t, "desc", cmd.Description)
}

func TestCreateRoleCommandRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	_, err := NewCreateRoleCommand("", "Admin", "tenant-a", "")
	require.Error(t, err)

	_, err = NewCreateRoleCommand("admin", "", "tenant-a", "")
	require.Error(t, err)

	_, err = NewCreateRoleCommand("admin", "Admin", "", "")
	require.Error(t, err)
}

func TestUpdateRoleCommandCopiesOptionalFields(t *testing.T) {
	t.Parallel()

	displayName := " Admin "
	description := "desc"
	cmd, err := NewUpdateRoleCommand(meta.FromUint64(11), &displayName, &description)

	require.NoError(t, err)
	require.Equal(t, uint64(11), cmd.ID.Uint64())
	require.NotNil(t, cmd.DisplayName)
	require.Equal(t, "Admin", *cmd.DisplayName)
	require.NotNil(t, cmd.Description)
	require.Equal(t, "desc", *cmd.Description)
}

func TestListRolesQueryUsesTenantValueObject(t *testing.T) {
	t.Parallel()

	query, err := NewListRolesQuery(" tenant-a ", 2, 5)

	require.NoError(t, err)
	require.Equal(t, "tenant-a", query.TenantIDString())
	require.Equal(t, 2, query.Offset)
	require.Equal(t, 5, query.Limit)
}
