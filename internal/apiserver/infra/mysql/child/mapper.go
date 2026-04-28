package child

import (
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ChildMapper 儿童档案映射器
// 负责领域模型与持久化对象之间的转换
type ChildMapper struct{}

// NewChildMapper 创建儿童档案映射器
func NewChildMapper() *ChildMapper {
	return &ChildMapper{}
}

// ToPO 将领域模型转换为持久化对象
func (m *ChildMapper) ToPO(cBO *domain.Child) *ChildPO {
	if cBO == nil {
		return nil
	}

	// 将空身份证号映射为 nil，以便在数据库中写入 NULL（避免 UNIQUE 索引与空字符串冲突）
	var idCardPtr *meta.IDCard
	if cBO.IDCard.String() != "" {
		idCardPtr = &cBO.IDCard
	}

	po := &ChildPO{
		Name:     cBO.Name,
		IDCard:   idCardPtr,
		Gender:   cBO.Gender.Value(),
		Birthday: cBO.Birthday.String(),
		Height:   cBO.Height.Tenths(),
		Weight:   cBO.Weight.Tenths(),
	}

	po.ID = cBO.ID

	return po
}

// ToBO 将持久化对象转换为领域模型
func (m *ChildMapper) ToBO(po *ChildPO) *domain.Child {
	if po == nil {
		return nil
	}

	var idCard meta.IDCard
	if po.IDCard != nil {
		idCard = *po.IDCard
	}

	child := &domain.Child{
		ID:       meta.ID(po.ID),
		Name:     po.Name,
		IDCard:   idCard,
		Gender:   meta.NewGender(po.Gender),
		Birthday: meta.NewBirthday(po.Birthday),
		Height:   meta.NewHeightFromTenths(po.Height),
		Weight:   meta.NewWeightFromTenths(po.Weight),
	}

	return child
}

// ToBOs 将持久化对象列表转换为领域模型列表
func (m *ChildMapper) ToBOs(pos []*ChildPO) []*domain.Child {
	if pos == nil {
		return nil
	}

	var bos []*domain.Child
	for _, po := range pos {
		bos = append(bos, m.ToBO(po))
	}

	return bos
}

// ToPOs 将领域模型列表转换为持久化对象列表
func (m *ChildMapper) ToPOs(bos []*domain.Child) []*ChildPO {
	if bos == nil {
		return nil
	}

	var pos []*ChildPO
	for _, bo := range bos {
		pos = append(pos, m.ToPO(bo))
	}

	return pos
}
