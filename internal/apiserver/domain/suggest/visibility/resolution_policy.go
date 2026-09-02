package visibility

import "slices"

// ResolutionPolicy 根据 Principal、授权事实与 visibility 生成最终 Scope。
type ResolutionPolicy struct{}

// Resolve 合并 Operator/Org/ProfileID 与手机号权限。
func (ResolutionPolicy) Resolve(
	principal Principal,
	facts AuthorizationFacts,
	visibilityProfileIDs []int64,
) Scope {
	if facts.PlatformListAllowed {
		return NewScope(true, facts.PlatformMobileSearchAllowed, 0, nil, nil)
	}

	orgIDs := principalOrgIDs(principal)
	var profileIDs []int64
	if len(visibilityProfileIDs) > 0 {
		profileIDs = mergeUniqueInt64(nil, visibilityProfileIDs)
	}
	return NewScope(false, facts.TenantMobileSearchAllowed, principal.OperatorID, orgIDs, profileIDs)
}

func principalOrgIDs(principal Principal) []int64 {
	if len(principal.OrgIDs) > 0 {
		return append([]int64(nil), principal.OrgIDs...)
	}
	if principal.OrgID > 0 {
		return []int64{principal.OrgID}
	}
	return nil
}

func mergeUniqueInt64(a, b []int64) []int64 {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(a)+len(b))
	for _, id := range a {
		if id <= 0 {
			continue
		}
		seen[id] = struct{}{}
	}
	for _, id := range b {
		if id <= 0 {
			continue
		}
		seen[id] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
