package runtime

import (
	"fmt"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRoleGraphResolvesDirectAndEffectiveRoles(t *testing.T) {
	t.Parallel()

	builder := newRoleGraphBuilder()
	tenantID := mustTenantID(t, "fangcun")
	otherTenant := mustTenantID(t, "other")
	user := mustSubject(t, subject.TypeUser, 42)
	group := mustSubject(t, subject.TypeGroup, 42)
	evaluator := mustRoleName(t, "qs:evaluator")
	planManager := mustRoleName(t, "qs:evaluation_plan_manager")
	staff := mustRoleName(t, "qs:staff")
	member := mustRoleName(t, "qs:member")

	assignments := []struct {
		sub      subject.Ref
		roleName role.Name
		tenantID tenant.ID
	}{
		{sub: user, roleName: evaluator, tenantID: tenantID},
		{sub: user, roleName: planManager, tenantID: tenantID},
		{sub: user, roleName: evaluator, tenantID: tenantID},
		{sub: group, roleName: member, tenantID: tenantID},
		{sub: user, roleName: member, tenantID: otherTenant},
	}
	for _, assignment := range assignments {
		builder.addAssignment(assignment.sub, assignment.roleName, assignment.tenantID)
	}
	inherited := []struct {
		child    role.Name
		parent   role.Name
		tenantID tenant.ID
	}{
		{child: evaluator, parent: staff, tenantID: tenantID},
		{child: planManager, parent: staff, tenantID: tenantID},
		{child: staff, parent: member, tenantID: tenantID},
		{child: staff, parent: member, tenantID: tenantID},
	}
	for _, edge := range inherited {
		builder.addInheritance(edge.child, edge.parent, edge.tenantID)
	}
	graph := builder.build(maxRoleHierarchyLevel)

	queries := []struct {
		name           string
		sub            subject.Ref
		tenantID       tenant.ID
		directRoles    []role.Name
		effectiveRoles []role.Name
	}{
		{
			name: "user", sub: user, tenantID: tenantID,
			directRoles:    []role.Name{planManager, evaluator},
			effectiveRoles: []role.Name{planManager, evaluator, member, staff},
		},
		{
			name: "group", sub: group, tenantID: tenantID,
			directRoles: []role.Name{member}, effectiveRoles: []role.Name{member},
		},
		{
			name: "other tenant", sub: user, tenantID: otherTenant,
			directRoles: []role.Name{member}, effectiveRoles: []role.Name{member},
		},
		{
			name: "unknown subject", sub: mustSubject(t, subject.TypeService, 99), tenantID: tenantID,
			directRoles: []role.Name{}, effectiveRoles: []role.Name{},
		},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			gotDirect, err := graph.DirectRoles(query.sub, query.tenantID)
			require.NoError(t, err)
			require.Equal(t, query.directRoles, gotDirect)
			require.NotNil(t, gotDirect)

			gotEffective, err := graph.EffectiveRoles(query.sub, query.tenantID)
			require.NoError(t, err)
			require.Equal(t, query.effectiveRoles, gotEffective)
			require.NotNil(t, gotEffective)
		})
	}
}

func TestRoleGraphPreservesMaximumHierarchyLevel(t *testing.T) {
	t.Parallel()

	builder := newRoleGraphBuilder()
	tenantID := mustTenantID(t, "fangcun")
	user := mustSubject(t, subject.TypeUser, 42)
	roles := make([]role.Name, maxRoleHierarchyLevel+2)
	for i := range roles {
		roles[i] = mustRoleName(t, fmt.Sprintf("qs:level_%02d", i))
	}
	builder.addAssignment(user, roles[0], tenantID)
	for i := 0; i < len(roles)-1; i++ {
		builder.addInheritance(roles[i], roles[i+1], tenantID)
	}

	got, err := builder.build(maxRoleHierarchyLevel).EffectiveRoles(user, tenantID)
	require.NoError(t, err)
	require.Equal(t, roles[:maxRoleHierarchyLevel], got)
}

func mustRoleName(t testing.TB, value string) role.Name {
	t.Helper()
	name, err := role.NewName(value)
	require.NoError(t, err)
	return name
}

func mustTenantID(t testing.TB, value string) tenant.ID {
	t.Helper()
	id, err := tenant.NewID(value)
	require.NoError(t, err)
	return id
}

func mustSubject(t testing.TB, subjectType subject.Type, id uint64) subject.Ref {
	t.Helper()
	ref, err := subject.NewRef(subjectType, meta.FromUint64(id))
	require.NoError(t, err)
	return ref
}
