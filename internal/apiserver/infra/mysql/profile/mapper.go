package profile

import (
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ProfileMapper 档案映射器
// 负责领域模型与持久化对象之间的转换
type ProfileMapper struct{}

// NewProfileMapper 创建档案映射器
func NewProfileMapper() *ProfileMapper {
	return &ProfileMapper{}
}

// ToPO 将领域模型转换为持久化对象
func (m *ProfileMapper) ToPO(cBO *domain.Profile) *ProfilePO {
	if cBO == nil {
		return nil
	}

	// 将空身份证号映射为 nil，以便在数据库中写入 NULL（避免 UNIQUE 索引与空字符串冲突）
	var idCardPtr *meta.IDCard
	if cBO.IDCard.String() != "" {
		idCardPtr = &cBO.IDCard
	}

	po := &ProfilePO{
		Name:     cBO.Name,
		IDCard:   idCardPtr,
		Gender:   cBO.Gender.Value(),
		Birthday: cBO.Birthday.String(),
	}

	po.ID = cBO.ID

	return po
}

// ToBO 将持久化对象转换为领域模型
func (m *ProfileMapper) ToBO(po *ProfilePO) *domain.Profile {
	if po == nil {
		return nil
	}

	var idCard meta.IDCard
	if po.IDCard != nil {
		idCard = *po.IDCard
	}

	profile := &domain.Profile{
		ID:       meta.ID(po.ID),
		Name:     po.Name,
		IDCard:   idCard,
		Gender:   meta.NewGender(po.Gender),
		Birthday: meta.NewBirthday(po.Birthday),
	}

	return profile
}

// ToBOs 将持久化对象列表转换为领域模型列表
func (m *ProfileMapper) ToBOs(pos []*ProfilePO) []*domain.Profile {
	if pos == nil {
		return nil
	}

	var bos []*domain.Profile
	for _, po := range pos {
		bos = append(bos, m.ToBO(po))
	}

	return bos
}

// ToPOs 将领域模型列表转换为持久化对象列表
func (m *ProfileMapper) ToPOs(bos []*domain.Profile) []*ProfilePO {
	if bos == nil {
		return nil
	}

	var pos []*ProfilePO
	for _, bo := range bos {
		pos = append(pos, m.ToPO(bo))
	}

	return pos
}
