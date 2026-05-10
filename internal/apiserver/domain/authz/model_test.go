package authz

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationBusinessModelsValidateRequiredFields(t *testing.T) {
	t.Parallel()

	_, err := NewSubject("", meta.FromUint64(100))
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	subject, err := NewSubject(SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	require.Equal(t, Subject{Type: SubjectTypeUser, ID: meta.FromUint64(100)}, subject)

	_, err = NewTenantScope("")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	permission, err := NewPermission(" iam:admin ", " tenant-a ", " iam:identity:collection:users ", " read ")
	require.NoError(t, err)
	require.Equal(t, Permission{
		RoleName:    "iam:admin",
		TenantID:    "tenant-a",
		ResourceKey: "iam:identity:collection:users",
		Action:      "read",
		Scope:       DefaultScope(),
	}, permission)

	originScope, err := NewScope(ScopeKindOrigin, "1")
	require.NoError(t, err)
	permission, err = NewPermission("iam:admin", "tenant-a", "iam:identity:collection:users", "read", WithPermissionScope(originScope))
	require.NoError(t, err)
	require.Equal(t, originScope, permission.Scope)

	binding, err := NewRoleBinding(subject, "iam:admin", "tenant-a", "operator-1")
	require.NoError(t, err)
	require.Equal(t, RoleBinding{
		Subject:   subject,
		RoleName:  "iam:admin",
		TenantID:  "tenant-a",
		GrantedBy: "operator-1",
	}, binding)

	request, err := NewAuthorizationRequest(subject, "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)
	require.Equal(t, AuthorizationRequest{
		Subject:     subject,
		TenantID:    "tenant-a",
		ResourceKey: "iam:identity:collection:users",
		Action:      "read",
		ObjectScope: DefaultScope(),
	}, request)

	request, err = NewAuthorizationRequest(subject, "tenant-a", "iam:identity:collection:users", "read", WithObjectScope(originScope))
	require.NoError(t, err)
	require.Equal(t, originScope, request.ObjectScope)
}

func TestScopeValidation(t *testing.T) {
	t.Parallel()

	scope, err := NewScope("", "")
	require.NoError(t, err)
	require.Equal(t, DefaultScope(), scope)
	require.Equal(t, "all:*", scope.String())

	scope, err = ParseScope("origin:1")
	require.NoError(t, err)
	require.Equal(t, Scope{Kind: ScopeKindOrigin, Value: "1"}, scope)

	_, err = NewScope(ScopeKindOrigin, "")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewScope(ScopeKindAll, "1")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = ParseScope("broken")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
