package casbin

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCasbinFactsMapFromAuthorizationBusinessModels(t *testing.T) {
	t.Parallel()

	subject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	permission, err := permission.New("iam:admin", "tenant-a", "iam:identity:collection:users", "read")
	require.NoError(t, err)
	binding, err := rolebinding.NewFact(subject, "iam:admin", "tenant-a", "operator-1")
	require.NoError(t, err)
	request, err := decision.NewRequest(subject, "tenant-a", "iam:identity:collection:users", "read")
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

func TestCasbinFactsPreserveActionPatternStrings(t *testing.T) {
	t.Parallel()

	permission, err := permission.New("qs:admin", "tenant-a", "qs:*:*:*", "read|list")
	require.NoError(t, err)
	rule := PolicyRuleFromPermission(permission)
	require.Equal(t, PolicyRule{
		Sub:   "role:qs:admin",
		Dom:   "tenant-a",
		Obj:   "qs:*:*:*",
		Act:   "read|list",
		Scope: "all:*",
	}, rule)

	restored, err := PermissionFromPolicyRule(PolicyRule{
		Sub:   "role:super_admin",
		Dom:   "platform",
		Obj:   "*:*:*:*",
		Act:   ".*",
		Scope: "all:*",
	})
	require.NoError(t, err)
	require.Equal(t, "super_admin", restored.RoleNameString())
	require.Equal(t, "platform", restored.TenantIDString())
	require.Equal(t, "*:*:*:*", restored.ResourceKeyString())
	require.Equal(t, ".*", restored.ActionString())
}
