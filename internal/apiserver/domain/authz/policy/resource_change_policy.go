package policy

import (
	"sort"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ResourceChangePolicy validates catalog changes against active permission grants.
type ResourceChangePolicy struct{}

// ValidateDependencies proves that every active grant depending on a catalog
// resource remains valid against the candidate catalog definition.
func (ResourceChangePolicy) ValidateDependencies(candidate resource.Resource, grants []*permissiongrant.Grant) error {
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

// EnsureUnused rejects deletion when active grants still reference the resource.
func (ResourceChangePolicy) EnsureUnused(grants []*permissiongrant.Grant) error {
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

// AffectedResourceTenantIDs returns a stable tenant order for version bumps.
// Resources are global catalog entries, so a compatible schema/action change
// must notify every tenant with a dependent active grant.
func (ResourceChangePolicy) AffectedResourceTenantIDs(changedByTenant string, grants []*permissiongrant.Grant) []string {
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
