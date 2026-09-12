package scopemigrate

import (
	"encoding/json"
	"fmt"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"sort"
)

// EffectivePermission describes the post-migration union for an existing catalog
// action. It is not a claim about record visibility under legacy QS rules.
type EffectivePermission struct {
	SubjectID     string     `json:"subject_id"`
	OrgID         string     `json:"org_id"`
	Resource      string     `json:"resource"`
	Action        string     `json:"action"`
	Kind          scope.Kind `json:"kind"`
	StoreIDs      []string   `json:"store_ids"`
	AssignmentIDs []string   `json:"assignment_ids"`
	GrantIDs      []string   `json:"grant_ids"`
}

func effectivePermissions(state rolemodel.State, changes []Change) ([]EffectivePermission, error) {
	targets := map[string][]Target{}
	for _, c := range changes {
		targets[c.AssignmentID] = c.Targets
	}
	roles := map[uint64]bool{}
	for _, r := range state.Roles {
		if r.DeletedAt == nil && governed(r.Name) {
			roles[r.ID.Uint64()] = true
		}
	}
	result := map[string]*EffectivePermission{}
	for _, a := range state.Assignments {
		if a.DeletedAt != nil || a.SubjectType != "user" || !roles[a.RoleID] {
			continue
		}
		assigned := targets[a.ID.String()]
		if len(assigned) == 0 {
			ids := []string{}
			if a.ScopeStoreIDs != nil {
				if err := json.Unmarshal([]byte(*a.ScopeStoreIDs), &ids); err != nil {
					return nil, err
				}
			}
			target, err := normalize(Target{OrgID: fmt.Sprint(a.OrgID), Kind: scope.Kind(a.ScopeKind), StoreIDs: ids})
			if err != nil {
				return nil, fmt.Errorf("assignment %s lacks valid scope: %w", a.ID, err)
			}
			assigned = []Target{target}
		}
		for _, g := range state.Grants {
			if g.RoleID != a.RoleID || g.DeletedAt != nil || g.RevokedAt != nil {
				continue
			}
			grant, err := (grantpo.Mapper{}).ToBO(&g)
			if err != nil {
				return nil, fmt.Errorf("grant %s: %w", g.ID, err)
			}
			for _, r := range state.Resources {
				if r.DeletedAt != nil {
					continue
				}
				resource, err := resourcepo.NewMapper().ToBO(&r)
				if err != nil {
					return nil, fmt.Errorf("resource %s: %w", r.ID, err)
				}
				if resource.Key.App() != "qs" || !grant.CoversResource(resource.Key) {
					continue
				}
				for _, action := range resource.Actions {
					if !grant.MatchesAction(action) {
						continue
					}
					for _, target := range assigned {
						key := a.SubjectID + "/" + target.OrgID + "/" + resource.KeyString() + "/" + action.String()
						row := result[key]
						if row == nil {
							row = &EffectivePermission{SubjectID: a.SubjectID, OrgID: target.OrgID, Resource: resource.KeyString(), Action: action.String(), Kind: scope.Stores, StoreIDs: []string{}, AssignmentIDs: []string{}, GrantIDs: []string{}}
							result[key] = row
						}
						if target.Kind == scope.AllStores {
							row.Kind = scope.AllStores
							row.StoreIDs = []string{}
						}
						if row.Kind != scope.AllStores {
							row.StoreIDs = unionStrings(row.StoreIDs, target.StoreIDs...)
						}
						row.AssignmentIDs = unionStrings(row.AssignmentIDs, a.ID.String())
						row.GrantIDs = unionStrings(row.GrantIDs, g.ID.String())
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]EffectivePermission, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, *result[key])
	}
	return rows, nil
}
func unionStrings(values []string, extra ...string) []string {
	seen := map[string]bool{}
	for _, v := range values {
		seen[v] = true
	}
	for _, v := range extra {
		seen[v] = true
	}
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
