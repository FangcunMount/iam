package profile

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

// CreationSpec 是档案创建的领域输入，应用层负责把外部 DTO 解析为值对象。
type CreationSpec struct {
	ID       meta.ID
	Name     string
	IDCard   meta.IDCard
	Gender   meta.Gender
	Birthday meta.Birthday
	Height   meta.Height
	Weight   meta.Weight
}

// NewFromCreationSpec 使用统一创建策略构造 Profile，集中档案创建字段的装配规则。
func NewFromCreationSpec(spec CreationSpec) (*Profile, error) {
	opts := []ProfileOption{
		WithGender(spec.Gender),
		WithBirthday(spec.Birthday),
		WithIDCard(spec.IDCard),
		WithHeight(spec.Height),
		WithWeight(spec.Weight),
	}
	if !spec.ID.IsZero() {
		opts = append(opts, WithProfileID(spec.ID))
	}
	return NewProfile(spec.Name, opts...)
}
