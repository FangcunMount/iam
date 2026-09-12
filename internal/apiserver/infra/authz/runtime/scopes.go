package runtime

import (
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"sort"
)

// permissionScopes accompanies one exported grant key/action. Consumers match
// resource/action patterns first, then union their paired ranges for a company.
func (s *Snapshot) permissionScopes(sub subject.Ref, resourceKey, action string) []scope.Scope {
	byCompany := map[meta.ID]scope.Scope{}
	for _, assignment := range s.assignmentsBySubject[sub.String()] {
		if assignment.Scope == nil {
			continue
		}
		for _, grant := range s.grantsByRole[assignment.RoleID] {
			if grant.ResourceKeyString() != resourceKey || grant.ActionString() != action {
				continue
			}
			value := *assignment.Scope
			if prior, ok := byCompany[value.OrgID()]; ok {
				// BuildSnapshot validated the immutable scopes; identical company is guaranteed.
				value, _ = prior.Union(value)
			}
			byCompany[value.OrgID()] = value
			break
		}
	}
	if len(byCompany) == 0 {
		return nil
	}
	result := make([]scope.Scope, 0, len(byCompany))
	for _, value := range byCompany {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OrgID() < result[j].OrgID() })
	return result
}
