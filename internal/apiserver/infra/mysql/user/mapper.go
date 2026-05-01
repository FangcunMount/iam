package user

import (
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
)

// UserMapper 用户映射器
// 负责领域模型与持久化对象之间的转换
type UserMapper struct{}

// NewUserMapper 创建用户映射器
func NewUserMapper() *UserMapper {
	return &UserMapper{}
}

// ToPO 将领域模型转换为持久化对象
func (m *UserMapper) ToPO(uBO *domain.User) *UserPO {
	if uBO == nil {
		return nil
	}

	po := &UserPO{
		Name:     uBO.Name,
		Nickname: uBO.Nickname,
		Phone:    uBO.Phone,
		Email:    uBO.Email,
		IDCard:   uBO.IDCard,
		Status:   uBO.Status.Value(),
	}

	// 设置嵌入字段的成员
	po.ID = uBO.ID

	return po
}

// ToBO 将持久化对象转换为领域模型
func (m *UserMapper) ToBO(po *UserPO) *domain.User {
	if po == nil {
		return nil
	}

	uBO, err := domain.NewUser(
		po.Name,
		po.Phone,
		domain.WithID(po.ID),
		domain.WithNickname(po.Nickname),
		domain.WithEmail(po.Email),
		domain.WithIDCard(po.IDCard),
		domain.WithStatus(domain.UserStatus(po.Status)),
	)
	if err != nil {
		return nil
	}

	return uBO
}

// ToBOs 将持久化对象列表转换为领域模型列表
func (m *UserMapper) ToBOs(pos []*UserPO) []*domain.User {
	if pos == nil {
		return nil
	}

	var bos []*domain.User
	for _, po := range pos {
		bos = append(bos, m.ToBO(po))
	}

	return bos
}

// ToPOs 将领域模型列表转换为持久化对象列表
func (m *UserMapper) ToPOs(bos []*domain.User) []*UserPO {
	if bos == nil {
		return nil
	}

	var pos []*UserPO
	for _, bo := range bos {
		pos = append(pos, m.ToPO(bo))
	}

	return pos
}
