package runtime

import (
	"sort"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type roleNameSet map[role.Name]struct{}

// roleGraphBuilder owns the mutable construction phase. build converts every
// set into a stable slice so the published roleGraph is read-only.
type roleGraphBuilder struct {
	directRoles    map[tenant.ID]map[subject.Ref]roleNameSet
	inheritedRoles map[tenant.ID]map[role.Name]roleNameSet
}

func newRoleGraphBuilder() *roleGraphBuilder {
	return &roleGraphBuilder{
		directRoles:    make(map[tenant.ID]map[subject.Ref]roleNameSet),
		inheritedRoles: make(map[tenant.ID]map[role.Name]roleNameSet),
	}
}

func (b *roleGraphBuilder) addAssignment(sub subject.Ref, roleName role.Name, tenantID tenant.ID) {
	if b.directRoles[tenantID] == nil {
		b.directRoles[tenantID] = make(map[subject.Ref]roleNameSet)
	}
	if b.directRoles[tenantID][sub] == nil {
		b.directRoles[tenantID][sub] = make(roleNameSet)
	}
	b.directRoles[tenantID][sub][roleName] = struct{}{}
}

func (b *roleGraphBuilder) addInheritance(child, parent role.Name, tenantID tenant.ID) {
	if b.inheritedRoles[tenantID] == nil {
		b.inheritedRoles[tenantID] = make(map[role.Name]roleNameSet)
	}
	if b.inheritedRoles[tenantID][child] == nil {
		b.inheritedRoles[tenantID][child] = make(roleNameSet)
	}
	b.inheritedRoles[tenantID][child][parent] = struct{}{}
}

func (b *roleGraphBuilder) build(maxHierarchyLevel int) *roleGraph {
	graph := &roleGraph{
		maxHierarchyLevel: maxHierarchyLevel,
		directRoles:       make(map[tenant.ID]map[subject.Ref][]role.Name, len(b.directRoles)),
		inheritedRoles:    make(map[tenant.ID]map[role.Name][]role.Name, len(b.inheritedRoles)),
	}
	for tenantID, rolesBySubject := range b.directRoles {
		graph.directRoles[tenantID] = make(map[subject.Ref][]role.Name, len(rolesBySubject))
		for sub, roles := range rolesBySubject {
			graph.directRoles[tenantID][sub] = sortedRoleNames(roles)
		}
	}
	for tenantID, parentsByRole := range b.inheritedRoles {
		graph.inheritedRoles[tenantID] = make(map[role.Name][]role.Name, len(parentsByRole))
		for child, parents := range parentsByRole {
			graph.inheritedRoles[tenantID][child] = sortedRoleNames(parents)
		}
	}
	return graph
}

// roleGraph is the immutable runtime projection of Subject -> Role and
// Role -> inherited Role facts for one published authorization snapshot.
type roleGraph struct {
	maxHierarchyLevel int
	directRoles       map[tenant.ID]map[subject.Ref][]role.Name
	inheritedRoles    map[tenant.ID]map[role.Name][]role.Name
}

var _ authorization.RoleResolver = (*roleGraph)(nil)

func (g *roleGraph) DirectRoles(sub subject.Ref, tenantID tenant.ID) ([]role.Name, error) {
	if g == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization role resolver is unavailable")
	}
	return cloneRoleNames(g.directRoles[tenantID][sub]), nil
}

func (g *roleGraph) EffectiveRoles(sub subject.Ref, tenantID tenant.ID) ([]role.Name, error) {
	if g == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization role resolver is unavailable")
	}
	if g.maxHierarchyLevel <= 0 {
		return make([]role.Name, 0), nil
	}

	frontier := g.directRoles[tenantID][sub]
	seen := make(roleNameSet)
	effective := make([]role.Name, 0, len(frontier))
	for level := 0; level < g.maxHierarchyLevel && len(frontier) > 0; level++ {
		next := make(roleNameSet)
		for _, current := range frontier {
			if _, exists := seen[current]; exists {
				continue
			}
			seen[current] = struct{}{}
			effective = append(effective, current)
		}
		for _, current := range frontier {
			for _, parent := range g.inheritedRoles[tenantID][current] {
				if _, exists := seen[parent]; !exists {
					next[parent] = struct{}{}
				}
			}
		}
		frontier = sortedRoleNames(next)
	}
	sort.Slice(effective, func(i, j int) bool {
		return effective[i].String() < effective[j].String()
	})
	return effective, nil
}

func sortedRoleNames(values roleNameSet) []role.Name {
	result := make([]role.Name, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].String() < result[j].String()
	})
	return result
}

func cloneRoleNames(values []role.Name) []role.Name {
	result := make([]role.Name, len(values))
	copy(result, values)
	return result
}
