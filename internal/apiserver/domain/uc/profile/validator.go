package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// validator 档案验证器实现
type validator struct {
	repo Repository
}

// NewValidator 创建档案验证器
func NewValidator(repo Repository) Validator {
	return &validator{repo: repo}
}

// ValidateCreate 验证创建参数
func (v *validator) ValidateCreate(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) error {
	// 验证名称
	if name == "" {
		return errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}

	return nil
}

// ValidateRename 验证改名参数
func (v *validator) ValidateRename(name string) error {
	if name == "" {
		return errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}
	return nil
}

// ValidateUpdateProfile 验证资料更新参数
func (v *validator) ValidateUpdateProfile(gender meta.Gender, birthday meta.Birthday) error {
	return nil
}
