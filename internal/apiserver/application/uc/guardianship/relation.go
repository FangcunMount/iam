package guardianship

import (
	"strings"

	gsshipdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
)

// ParseRelation 将外部输入统一映射到领域层的监护关系词表。
func ParseRelation(relation string) gsshipdomain.Relation {
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case "self":
		return gsshipdomain.RelSelf
	case "parent":
		return gsshipdomain.RelParent
	case "grandparent":
		return gsshipdomain.RelGrandparent
	case "other":
		return gsshipdomain.RelOther
	default:
		return gsshipdomain.RelOther
	}
}

// NormalizeRelation 将输入标准化为对外统一返回的 relation 文本。
func NormalizeRelation(relation string) string {
	return string(ParseRelation(relation))
}
