package shared

import (
	"sort"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ValidateResourceDependencies proves that every active grant depending on a
// catalog resource remains valid against the candidate catalog definition.
func ValidateResourceDependencies(candidate resource.Resource, grants []*permissiongrant.Grant) error {
	for _, grant := range grants {
		if grant == nil || !grant.IsActive() {
			continue
		}
		if err := grant.ValidateAgainst(candidate); err != nil {
			return perrors.WithCode(code.ErrResourceInUse, "resource change would invalidate permission grant %s: %v", grant.ID.String(), err)
		}
	}
	return nil
}

func EnsureResourceUnused(grants []*permissiongrant.Grant) error {
	active := 0
	for _, grant := range grants {
		if grant != nil && grant.IsActive() {
			active++
		}
	}
	if active > 0 {
		return perrors.WithCode(code.ErrResourceInUse, "resource has %d active permission grants", active)
	}
	return nil
}

func EnsureRoleUnused(roleID meta.ID, bindings []*rolebinding.Binding, grants []*permissiongrant.Grant, inheritances []*roleinheritance.Inheritance) error {
	if len(bindings) > 0 {
		return perrors.WithCode(code.ErrRoleInUse, "role has %d active assignments", len(bindings))
	}
	for _, grant := range grants {
		if grant != nil && grant.IsActive() {
			return perrors.WithCode(code.ErrRoleInUse, "role has active permission grants")
		}
	}
	for _, inheritance := range inheritances {
		if inheritance != nil && inheritance.IsActive() && (inheritance.RoleID == roleID || inheritance.InheritedRoleID == roleID) {
			return perrors.WithCode(code.ErrRoleInUse, "role participates in an active inheritance")
		}
	}
	return nil
}

// AffectedResourceTenantIDs returns a stable tenant order for version bumps.
// Resources are global catalog entries, so a compatible schema/action change
// must notify every tenant with a dependent active grant.
func AffectedResourceTenantIDs(changedByTenant string, grants []*permissiongrant.Grant) []string {
	seen := make(map[string]struct{}, len(grants)+1)
	add := func(tenantID string) {
		tenantID = strings.TrimSpace(tenantID)
		if tenantID != "" {
			seen[tenantID] = struct{}{}
		}
	}
	add(changedByTenant)
	for _, grant := range grants {
		if grant != nil && grant.IsActive() {
			add(grant.TenantIDString())
		}
	}
	result := make([]string, 0, len(seen))
	for tenantID := range seen {
		result = append(result, tenantID)
	}
	sort.Strings(result)
	return result
}
