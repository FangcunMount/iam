package suggest

import (
	"strings"
	"unicode"
)

const (
	DefaultLimit              = 20
	DefaultKeyPadLen          = 25
	DefaultInternalMult       = 10
	DefaultWildcardKeyCap = 100
)

// ProfileSearchTerm 是索引中的档案读模型项（搜索字段 + 权限过滤最小维度）。
type ProfileSearchTerm struct {
	ProfileID        int64
	DisplayName      string
	Mobiles          []string
	Weight int
	// TenantID Deprecated: 预留给未来 SaaS 授权域隔离；suggest 数据权限勿依赖。
	TenantID int64
	// OrgID 业务组织可见范围（读模型字段，来自业务侧/Loader，非 IAM 核心身份概念）。
	OrgID            int64
	OwnerOperatorIDs []int64
}

// NewProfileSearchTerm 构建并规范化 ProfileSearchTerm。
func NewProfileSearchTerm(
	profileID int64,
	displayName string,
	mobiles []string,
	weight int,
	tenantID int64,
	orgID int64,
	ownerOperatorIDs []int64,
) ProfileSearchTerm {
	cleanMobiles := make([]string, 0, len(mobiles))
	for _, mobile := range mobiles {
		mobile = strings.TrimSpace(mobile)
		if mobile == "" {
			continue
		}
		cleanMobiles = append(cleanMobiles, mobile)
	}
	return ProfileSearchTerm{
		ProfileID:        profileID,
		DisplayName:      strings.TrimSpace(displayName),
		Mobiles:          cleanMobiles,
		Weight:           weight,
		TenantID:         tenantID,
		OrgID:            orgID,
		OwnerOperatorIDs: uniqueInt64(ownerOperatorIDs),
	}
}

func uniqueInt64(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Keyword 表达一次档案联想查询的关键字。
type Keyword struct {
	value string
}

func NewKeyword(value string) Keyword {
	return Keyword{value: strings.TrimSpace(value)}
}

func (k Keyword) String() string {
	return k.value
}

func (k Keyword) IsDigits() bool {
	if k.value == "" {
		return false
	}
	for _, r := range k.value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Query 表达一次档案联想查询及其限制。
type Query struct {
	Keyword            Keyword
	Limit              int
	InternalLimit      int
	KeyPadLen          int
	WildcardKeyCap int // 通配符展开的最大终端键数量；<=0 使用 DefaultWildcardKeyCap。
}

// NewQuery 构造 Query；wildcardKeyCap<=0 时使用 DefaultWildcardKeyCap。
func NewQuery(keyword string, limit, internalLimit, keyPadLen, wildcardKeyCap int) Query {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if internalLimit <= 0 {
		internalLimit = limit * DefaultInternalMult
	}
	if internalLimit < limit {
		internalLimit = limit
	}
	if keyPadLen <= 0 {
		keyPadLen = DefaultKeyPadLen
	}
	if wildcardKeyCap <= 0 {
		wildcardKeyCap = DefaultWildcardKeyCap
	}
	return Query{
		Keyword:        NewKeyword(keyword),
		Limit:          limit,
		InternalLimit:  internalLimit,
		KeyPadLen:      keyPadLen,
		WildcardKeyCap: wildcardKeyCap,
	}
}
