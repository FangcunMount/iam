package user

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// User 基础用户（身份锚点）
type User struct {
	ID       meta.ID
	Name     string
	Nickname string
	Phone    meta.Phone
	Email    meta.Email
	IDCard   meta.IDCard
	Status   UserStatus
}

// NewUser 创建新用户（完整信息）
func NewUser(name string, phone meta.Phone, opts ...UserOption) (*User, error) {
	if name == "" {
		return nil, errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}

	user := &User{
		Name:   name,
		Phone:  phone,
		Status: UserActive, // 新用户默认为活跃状态
	}
	for _, opt := range opts {
		opt(user)
	}

	return user, nil
}

// UserOption 用户选项，用于创建用户时的可选参数
type UserOption func(*User)

// With*** 用户选项函数
func WithID(id meta.ID) UserOption             { return func(u *User) { u.ID = id } }
func WithNickname(nickname string) UserOption  { return func(u *User) { u.Nickname = nickname } }
func WithEmail(email meta.Email) UserOption    { return func(u *User) { u.Email = email } }
func WithIDCard(idCard meta.IDCard) UserOption { return func(u *User) { u.IDCard = idCard } }
func WithStatus(status UserStatus) UserOption  { return func(u *User) { u.Status = status } }

// UserStatus 用户状态
func (u *User) Activate()   { u.Status = UserActive }
func (u *User) Deactivate() { u.Status = UserInactive }
func (u *User) Block()      { u.Status = UserBlocked }

// 检查用户状态
func (u *User) IsUsable() bool   { return u.Status == UserActive }
func (u *User) IsBlocked() bool  { return u.Status == UserBlocked }
func (u *User) IsInactive() bool { return u.Status == UserInactive }

// Rename 更新用户名
func (u *User) Rename(name string) { u.Name = name }

// UpdateNickname 更新昵称
func (u *User) UpdateNickname(nickname string) { u.Nickname = nickname }

// UpdatePhone 修改电话
func (u *User) UpdatePhone(phone meta.Phone) {
	u.Phone = phone
}

// UpdateEmail 修改邮箱
func (u *User) UpdateEmail(email meta.Email) {
	u.Email = email
}

// UpdateIDCard 更新身份证
func (u *User) UpdateIDCard(idc meta.IDCard) { u.IDCard = idc }
