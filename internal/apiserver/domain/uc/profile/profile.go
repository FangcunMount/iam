package profile

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Profile 档案（档案）
type Profile struct {
	ID       meta.ID
	Name     string
	IDCard   meta.IDCard
	Gender   meta.Gender
	Birthday meta.Birthday
	Height   meta.Height
	Weight   meta.Weight
}

func NewProfile(name string, opts ...ProfileOption) (*Profile, error) {
	if name == "" {
		return nil, errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}

	profile := &Profile{Name: name}
	for _, opt := range opts {
		opt(profile)
	}

	return profile, nil
}

// ProfileOption 档案选项，用于创建档案时的可选参数
type ProfileOption func(*Profile)

// With*** 档案选项函数
func WithProfileID(id meta.ID) ProfileOption      { return func(c *Profile) { c.ID = id } }
func WithIDCard(idCard meta.IDCard) ProfileOption { return func(c *Profile) { c.IDCard = idCard } }
func WithGender(gender meta.Gender) ProfileOption { return func(c *Profile) { c.Gender = gender } }
func WithBirthday(birthday meta.Birthday) ProfileOption {
	return func(c *Profile) { c.Birthday = birthday }
}
func WithHeight(height meta.Height) ProfileOption { return func(c *Profile) { c.Height = height } }
func WithWeight(weight meta.Weight) ProfileOption { return func(c *Profile) { c.Weight = weight } }

// Rename 重命名
func (c *Profile) Rename(name string) { c.Name = name }

// UpdateIDCard 更新身份证
func (c *Profile) UpdateIDCard(idc meta.IDCard) { c.IDCard = idc }

// UpdateProfile 更新基本信息
func (c *Profile) UpdateProfile(g meta.Gender, d meta.Birthday) {
	c.Gender, c.Birthday = g, d
}

// UpdateHeight 更新身高体重
func (c *Profile) UpdateHeightWeight(h meta.Height, w meta.Weight) {
	c.Height, c.Weight = h, w
}
