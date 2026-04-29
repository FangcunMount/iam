package profilelink

import (
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
)

// ProfileLinkMapper 档案关系映射器
type ProfileLinkMapper struct{}

// NewProfileLinkMapper 创建档案关系映射器
func NewProfileLinkMapper() *ProfileLinkMapper {
	return &ProfileLinkMapper{}
}

// ToPO 将领域模型转换为持久化对象
func (m *ProfileLinkMapper) ToPO(gBO *domain.ProfileLink) *ProfileLinkPO {
	if gBO == nil {
		return nil
	}

	po := &ProfileLinkPO{
		UserID:        gBO.User,
		ProfileID:     gBO.Profile,
		Type:          string(gBO.Type),
		Relation:      string(gBO.Rel),
		EstablishedAt: gBO.EstablishedAt,
		RevokedAt:     gBO.RevokedAt,
	}

	if !gBO.ID.IsZero() {
		po.ID = gBO.ID
	}

	return po
}

// ToBO 将持久化对象转换为领域模型
func (m *ProfileLinkMapper) ToBO(po *ProfileLinkPO) *domain.ProfileLink {
	if po == nil {
		return nil
	}

	gBO := &domain.ProfileLink{
		ID:            po.ID,
		User:          po.UserID,
		Profile:       po.ProfileID,
		Type:          domain.Type(po.Type),
		Rel:           domain.Relation(po.Relation),
		EstablishedAt: po.EstablishedAt,
		RevokedAt:     po.RevokedAt,
	}

	return gBO
}

// ToBOs 将持久化对象列表转换为领域模型列表
func (m *ProfileLinkMapper) ToBOs(pos []*ProfileLinkPO) []*domain.ProfileLink {
	if pos == nil {
		return nil
	}

	var bos []*domain.ProfileLink
	for _, po := range pos {
		bos = append(bos, m.ToBO(po))
	}

	return bos
}

// ToPOs 将领域模型列表转换为持久化对象列表
func (m *ProfileLinkMapper) ToPOs(bos []*domain.ProfileLink) []*ProfileLinkPO {
	if bos == nil {
		return nil
	}

	var pos []*ProfileLinkPO
	for _, bo := range bos {
		pos = append(pos, m.ToPO(bo))
	}

	return pos
}
