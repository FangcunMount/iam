package suggest

// ProfileAccessScope 表示权限侧解析后的 Profile 可见范围（不是权限规则本身）。
type ProfileAccessScope struct {
	AllProfile        bool
	OrgIDs            []int64 // 业务组织可见范围，非 IAM tenant。
	OperatorID        int64
	ProfileIDs        []int64
	AllowMobileSearch bool
}

// CompiledProfileAccessScope 将 ProfileAccessScope 中的切片编译为集合，适合高频 Allows。
type CompiledProfileAccessScope struct {
	AllProfile        bool
	AllowMobileSearch bool
	OperatorID        int64
	profileSet        map[int64]struct{}
	orgSet            map[int64]struct{}
}

// CompileProfileAccessScope 编译 scope；对单次 suggest 查询 reuse 同一编译结果即可。
func CompileProfileAccessScope(s ProfileAccessScope) CompiledProfileAccessScope {
	return CompiledProfileAccessScope{
		AllProfile:        s.AllProfile,
		AllowMobileSearch: s.AllowMobileSearch,
		OperatorID:        s.OperatorID,
		profileSet:        int64SliceToSet(s.ProfileIDs),
		orgSet:            int64SliceToSet(s.OrgIDs),
	}
}

func int64SliceToSet(ids []int64) map[int64]struct{} {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		m[id] = struct{}{}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// ScopePolicy 在内存中对候选项做轻量范围判断（不调 AuthZ）。
type ScopePolicy struct{}

// Allows 判断 term 是否落在 scope 内。
func (p ScopePolicy) Allows(scope ProfileAccessScope, term ProfileSearchTerm) bool {
	return p.AllowsCompiled(CompileProfileAccessScope(scope), term)
}

// AllowsCompiled 使用编译后的 scope 判断可见性。
func (ScopePolicy) AllowsCompiled(c CompiledProfileAccessScope, term ProfileSearchTerm) bool {
	if c.AllProfile {
		return true
	}
	if c.profileSet != nil {
		if _, ok := c.profileSet[term.ProfileID]; ok {
			return true
		}
	}
	if c.orgSet != nil && term.OrgID > 0 {
		if _, ok := c.orgSet[term.OrgID]; ok {
			return true
		}
	}
	if c.OperatorID > 0 && containsInt64(term.OwnerOperatorIDs, c.OperatorID) {
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
