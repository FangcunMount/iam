package profilelink

import gsshipdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profilelink"

// ParseRelation 将外部输入统一映射到领域层的档案关系词表。
func ParseRelation(relation string) gsshipdomain.Relation {
	return gsshipdomain.ParseRelation(relation)
}

// NormalizeRelation 将输入标准化为对外统一返回的 relation 文本。
func NormalizeRelation(relation string) string {
	return gsshipdomain.NormalizeRelation(relation)
}
