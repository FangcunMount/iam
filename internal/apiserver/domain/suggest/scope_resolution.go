package suggest

import "slices"

// ProfileAuthorizationFacts 是 Suggest 所需的粗粒度授权事实。
type ProfileAuthorizationFacts struct {
	PlatformListAllowed         bool
	PlatformMobileSearchAllowed bool
	TenantMobileSearchAllowed   bool
}

// ScopeResolutionPolicy 根据 Principal、授权事实与 visibility 生成最终 Scope。
type ScopeResolutionPolicy struct{}

// Resolve 合并 Operator/Org/ProfileID 与手机号权限。
func (ScopeResolutionPolicy) Resolve(
	principal OperatingPrincipal,
	facts ProfileAuthorizationFacts,
	visibilityProfileIDs []int64,
) ProfileAccessScope {
	if facts.PlatformListAllowed {
		return ProfileAccessScope{
			AllProfile:        true,
			AllowMobileSearch: facts.PlatformMobileSearchAllowed,
		}
	}

	mobileOK := facts.TenantMobileSearchAllowed
	out := ProfileAccessScope{
		OperatorID:        principal.OperatorID,
		OrgIDs:            principalOrgIDs(principal),
		AllowMobileSearch: mobileOK,
	}
	if len(visibilityProfileIDs) > 0 {
		out.ProfileIDs = mergeUniqueInt64(nil, visibilityProfileIDs)
	}
	return out
}

func principalOrgIDs(principal OperatingPrincipal) []int64 {
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
