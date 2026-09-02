package visibility

// Scope 表示权限侧解析后的 Profile 可见范围。
type Scope struct {
	allProfiles      bool
	mobileSearch     bool
	operatorID       int64
	orgIDs           map[int64]struct{}
	profileIDs       map[int64]struct{}
}

// NewScope 构造不可随意修改的 Scope。
func NewScope(allProfiles, mobileSearch bool, operatorID int64, orgIDs, profileIDs []int64) Scope {
	return Scope{
		allProfiles:  allProfiles,
		mobileSearch: mobileSearch,
		operatorID:   operatorID,
		orgIDs:       int64SliceToSet(orgIDs),
		profileIDs:   int64SliceToSet(profileIDs),
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

// Allows 判断 resource 是否在 scope 内可见。
func (s Scope) Allows(resource Resource) bool {
	if s.allProfiles {
		return true
	}
	if s.profileIDs != nil {
		if _, ok := s.profileIDs[resource.ProfileID]; ok {
			return true
		}
	}
	if s.orgIDs != nil && resource.OrgID > 0 {
		if _, ok := s.orgIDs[resource.OrgID]; ok {
			return true
		}
	}
	if s.operatorID > 0 && containsInt64(resource.OwnerOperatorIDs, s.operatorID) {
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

func (s Scope) AllowsMobileSearch() bool { return s.mobileSearch }
func (s Scope) IsAllProfiles() bool      { return s.allProfiles }
func (s Scope) OperatorID() int64        { return s.operatorID }

// OrgIDs 返回去重后的 org ID 切片副本。
func (s Scope) OrgIDs() []int64       { return setToSortedSlice(s.orgIDs) }
func (s Scope) ProfileIDs() []int64   { return setToSortedSlice(s.profileIDs) }

func setToSortedSlice(m map[int64]struct{}) []int64 {
	if len(m) == 0 {
		return nil
	}
	out := make([]int64, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}
