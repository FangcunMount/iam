package native

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCasbinRoleResolverSeparatesDirectAndEffectiveRolesByTenant(t *testing.T) {
	t.Parallel()

	resolver := newCasbinRoleResolver(maxRoleHierarchyLevel)
	sub, err := subject.NewUserRef(meta.FromUint64(42))
	require.NoError(t, err)
	tenantID, err := tenant.NewID("fangcun")
	require.NoError(t, err)
	otherTenant, err := tenant.NewID("other")
	require.NoError(t, err)
	child, err := role.NewName("qs:evaluator")
	require.NoError(t, err)
	parent, err := role.NewName("qs:staff")
	require.NoError(t, err)

	require.NoError(t, resolver.addAssignment(sub, child, tenantID))
	require.NoError(t, resolver.addInheritance(child, parent, tenantID))

	direct, err := resolver.DirectRoles(sub, tenantID)
	require.NoError(t, err)
	require.Equal(t, []role.Name{child}, direct)
	effective, err := resolver.EffectiveRoles(sub, tenantID)
	require.NoError(t, err)
	require.Equal(t, []role.Name{child, parent}, effective)
	other, err := resolver.EffectiveRoles(sub, otherTenant)
	require.NoError(t, err)
	require.Empty(t, other)
}
