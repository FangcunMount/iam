package user

import (
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// User 基础用户（身份锚点）
type User struct {
	ID meta.ID

	Name     string
	Nickname string
	Phone    meta.Phone
	Email    meta.Email

	Status Status // 用户状态: 1 活跃；2 非活跃；3 被封禁
}

// NewUser 创建用户（完整信息）
func NewUser(name string, phone meta.Phone, opts ...UserOption) (*User, error) {
	normalizedName, err := normalizeAndValidateUserName(name)
	if err != nil {
		return nil, err
	}
	u := &User{
		Name:   normalizedName,
		Phone:  phone,
		Status: UserActive,
	}
	for _, opt := range opts {
		opt(u)
	}
	if !u.Status.IsValid() {
		return nil, errors.WithCode(code.ErrUserBasicInfoInvalid, "invalid user status")
	}
	return u, nil

}

// UserOption 用户选项，用于创建用户时的可选参数
type UserOption func(*User)

// With*** 用户选项函数
func WithID(id meta.ID) UserOption { return func(u *User) { u.ID = id } }
func WithNickname(nickname string) UserOption {
	return func(u *User) { u.Nickname = strings.TrimSpace(nickname) }
}
func WithEmail(email meta.Email) UserOption { return func(u *User) { u.Email = email } }
func WithStatus(status Status) UserOption   { return func(u *User) { u.Status = status } }

// UserStatus 用户状态
func (u *User) Activate()   { u.changeStatus(UserActive) }
func (u *User) Deactivate() { u.changeStatus(UserInactive) }
func (u *User) Block()      { u.changeStatus(UserBlocked) }
func (u *User) changeStatus(toStatus Status) {
	u.Status = toStatus
}

// 检查用户状态
func (u *User) IsUsable() bool         { return u.isStatus(UserActive) }
func (u *User) IsBlocked() bool        { return u.isStatus(UserBlocked) }
func (u *User) IsInactive() bool       { return u.isStatus(UserInactive) }
func (u *User) isStatus(s Status) bool { return u.Status == s }

// Rename 更新用户名
func (u *User) Rename(name string) error {
	normalizedName, err := normalizeAndValidateUserName(name)
	if err != nil {
		return err
	}
	u.Name = normalizedName
	return nil
}

// ChangeNickname 更新昵称
func (u *User) ChangeNickname(nickname string) { u.Nickname = nickname }

// ChangePhone 修改电话
func (u *User) ChangePhone(phone meta.Phone) { u.Phone = phone }

// ChangeEmail 修改邮箱
func (u *User) ChangeEmail(email meta.Email) { u.Email = email }

func normalizeAndValidateUserName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}
	return name, nil

}
