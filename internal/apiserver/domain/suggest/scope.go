package suggest

// ProfileAccessScope 表示权限侧解析后的 Profile 可见范围（不是权限规则本身）。
type ProfileAccessScope struct {
	AllProfile        bool
	TenantIDs         []int64
	OrgIDs            []int64
	OperatorID        int64
	ProfileIDs        []int64
	AllowMobileSearch bool
}

// ScopePolicy 在内存中对候选项做轻量范围判断（不调 AuthZ）。
type ScopePolicy struct{}

// Allows 判断 term 是否落在 scope 内。
func (ScopePolicy) Allows(scope ProfileAccessScope, term ProfileSearchTerm) bool {
	if scope.AllProfile {
		return true
	}
	if containsInt64(scope.ProfileIDs, term.ProfileID) {
		return true
	}
	if containsInt64(scope.TenantIDs, term.TenantID) {
		return true
	}
	if containsInt64(scope.OrgIDs, term.OrgID) {
		return true
	}
	if scope.OperatorID > 0 && containsInt64(term.OwnerOperatorIDs, scope.OperatorID) {
		return true
	}
	return false
}

func containsInt64(haystack []int64, needle int64) bool {
	if needle == 0 {
		return false
	}
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
