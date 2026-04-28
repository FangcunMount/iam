package child

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Child 孩子（儿童档案）
type Child struct {
	ID       meta.ID
	Name     string
	IDCard   meta.IDCard
	Gender   meta.Gender
	Birthday meta.Birthday
	Height   meta.Height
	Weight   meta.Weight
}

func NewChild(name string, opts ...ChildOption) (*Child, error) {
	if name == "" {
		return nil, errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}

	child := &Child{Name: name}
	for _, opt := range opts {
		opt(child)
	}

	return child, nil
}

// ChildOption 儿童档案选项，用于创建儿童档案时的可选参数
type ChildOption func(*Child)

// With*** 儿童档案选项函数
func WithChildID(id meta.ID) ChildOption        { return func(c *Child) { c.ID = id } }
func WithIDCard(idCard meta.IDCard) ChildOption { return func(c *Child) { c.IDCard = idCard } }
func WithGender(gender meta.Gender) ChildOption { return func(c *Child) { c.Gender = gender } }
func WithBirthday(birthday meta.Birthday) ChildOption {
	return func(c *Child) { c.Birthday = birthday }
}
func WithHeight(height meta.Height) ChildOption { return func(c *Child) { c.Height = height } }
func WithWeight(weight meta.Weight) ChildOption { return func(c *Child) { c.Weight = weight } }

// Rename 重命名
func (c *Child) Rename(name string) { c.Name = name }

// UpdateIDCard 更新身份证
func (c *Child) UpdateIDCard(idc meta.IDCard) { c.IDCard = idc }

// UpdateProfile 更新基本信息
func (c *Child) UpdateProfile(g meta.Gender, d meta.Birthday) {
	c.Gender, c.Birthday = g, d
}

// UpdateHeight 更新身高体重
func (c *Child) UpdateHeightWeight(h meta.Height, w meta.Weight) {
	c.Height, c.Weight = h, w
}
