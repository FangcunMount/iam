package profilelink

import "strings"

// 档案关系
type Relation string

const (
	RelSelf        Relation = "self"        // 自己
	RelParent      Relation = "parent"      // 父母
	RelGrandparent Relation = "grandparent" // 祖父母
	RelOther       Relation = "other"       // 其他
)

// Type 描述关系边的主类别，Relation 描述同一类别下的业务关系。
type Type string

const (
	TypeSelf     Type = "self"
	TypeRelation Type = "relation"
)

// ParseRelation maps external text to the profile link domain vocabulary.
func ParseRelation(relation string) Relation {
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case "self":
		return RelSelf
	case "parent":
		return RelParent
	case "grandparent":
		return RelGrandparent
	case "other":
		return RelOther
	default:
		return RelOther
	}
}

func TypeFromRelation(relation Relation) Type {
	if relation == RelSelf {
		return TypeSelf
	}
	return TypeRelation
}
