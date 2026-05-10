package casbin

import (
	"testing"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCasbinFactsMapFromAuthorizationBusinessModels(t *testing.T) {
	t.Parallel()

	subject, err := authzDomain.NewSubject(authzDomain.SubjectTypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	permission, err := authzDomain.NewPermission("iam:admin", "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)
	binding, err := authzDomain.NewRoleBinding(subject, "iam:admin", "tenant-a", "operator-1")
	require.NoError(t, err)
	request, err := authzDomain.NewAuthorizationRequest(subject, "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)

	require.Equal(t, Request{
		Sub:   "user:100",
		Dom:   "tenant-a",
		Obj:   "iam:identity:collection:users",
		Act:   "read",
		Scope: "all:*",
	}, RequestFromAuthorizationRequest(request))

	require.Equal(t, PolicyRule{
		Sub:   "role:iam:admin",
		Dom:   "tenant-a",
		Obj:   "iam:identity:collection:users",
		Act:   "read",
		Scope: "all:*",
	}, PolicyRuleFromPermission(permission))

	require.Equal(t, GroupingRule{
		Sub:  "user:100",
		Role: "role:iam:admin",
		Dom:  "tenant-a",
	}, GroupingRuleFromRoleBinding(binding))

	require.Equal(t, "iam:admin", RoleNameFromKey("role:iam:admin"))
}
