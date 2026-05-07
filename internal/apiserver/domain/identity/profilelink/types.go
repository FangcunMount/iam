package profilelink

import (
	"strings"
)

// Type 类型：档案类型
type Type string

// Relation: User 与 Profile 的关系
type Relation string

// Type 常量
const (
	TypeSelf     Type = "self"
	TypeRelation Type = "relation"
)

// Relation 常量
const (
	RelSelf        Relation = "self"
	RelParent      Relation = "parent"
	RelGrandparent Relation = "grandparent"
	RelOther       Relation = "other"
)

// IsValid Relation 值是否合法
func (r Relation) IsValid() bool {
	switch r {
	case RelSelf, RelParent, RelGrandparent, RelOther:
		return true
	default:
		return false
	}
}

// ParseRelation 将 string 解析为 Relation
func ParseRelation(relStr string) Relation {
	relationMap := map[string]Relation{
		"self":        RelSelf,
		"parent":      RelParent,
		"grandparent": RelGrandparent,
	}

	if rel, ok := relationMap[strings.ToLower(strings.TrimSpace(relStr))]; !ok {
		return RelOther
	} else {
		return rel
	}
}

// TypeFromRelation 由 Relation 转为 Type
func TypeFromRelation(rel Relation) Type {
	if rel == RelSelf {
		return TypeSelf
	}

	return TypeRelation
}
