package suggest

import (
	"sort"
	"strings"
	"unicode"
)

const (
	DefaultLimit     = 20
	DefaultKeyPadLen = 25
)

// ProfileCandidate 是档案联想索引的领域输入候选。
type ProfileCandidate struct {
	ProfileID   int64
	DisplayName string
	Mobiles     []string
	Weight      int
}

func NewProfileCandidate(profileID int64, displayName string, mobiles []string, weight int) ProfileCandidate {
	cleanMobiles := make([]string, 0, len(mobiles))
	for _, mobile := range mobiles {
		mobile = strings.TrimSpace(mobile)
		if mobile == "" {
			continue
		}
		cleanMobiles = append(cleanMobiles, mobile)
	}
	return ProfileCandidate{
		ProfileID:   profileID,
		DisplayName: strings.TrimSpace(displayName),
		Mobiles:     cleanMobiles,
		Weight:      weight,
	}
}

func (c ProfileCandidate) Term() Term {
	return Term{
		Name:   c.DisplayName,
		ID:     c.ProfileID,
		Mobile: strings.Join(c.Mobiles, ","),
		Weight: c.Weight,
	}
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
	Keyword   Keyword
	Limit     int
	KeyPadLen int
}

func NewQuery(keyword string, limit int, keyPadLen int) Query {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if keyPadLen <= 0 {
		keyPadLen = DefaultKeyPadLen
	}
	return Query{
		Keyword:   NewKeyword(keyword),
		Limit:     limit,
		KeyPadLen: keyPadLen,
	}
}

// RankingPolicy 保持档案联想结果的去重与排序规则。
type RankingPolicy struct{}

func (RankingPolicy) Rank(terms []Term, limit int) []Term {
	if limit <= 0 {
		limit = DefaultLimit
	}
	seen := make(map[int64]struct{}, len(terms))
	unique := make([]Term, 0, len(terms))
	for _, term := range terms {
		if _, exists := seen[term.ID]; exists {
			continue
		}
		seen[term.ID] = struct{}{}
		unique = append(unique, term)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Weight > unique[j].Weight
	})
	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}
