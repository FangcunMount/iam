package user

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// validator 用户验证器（领域服务）
// 封装用户相关的验证规则和业务检查
type validator struct {
	repo Repository
}

// 确保 validator 实现了 Validator 接口
var _ Validator = (*validator)(nil)

// NewValidator 创建用户验证器
func NewValidator(repo Repository) Validator {
	return &validator{repo: repo}
}

// ValidateRegister 验证注册参数
// 检查手机号唯一性和基本参数有效性
func (v *validator) ValidateRegister(ctx context.Context, name string, phone meta.Phone) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return perrors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}

	// 检查手机号唯一性
	if !phone.IsEmpty() {
		return v.CheckPhoneUnique(ctx, phone)
	}
	return nil
}

// ValidateRename 验证改名参数
func (v *validator) ValidateRename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return perrors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
	}
	return nil
}

// ValidateUpdateContact 验证更新联系方式参数
// 如果手机号变更，需要检查唯一性
func (v *validator) ValidateUpdateContact(ctx context.Context, user *User, phone meta.Phone, email meta.Email) error {
	// 如果手机号变更，检查唯一性
	if !phone.IsEmpty() && !user.Phone.Equal(phone) {
		if err := v.CheckPhoneUnique(ctx, phone); err != nil {
			return err
		}
	}
	return nil
}

// CheckPhoneUnique 检查手机号唯一性
func (v *validator) CheckPhoneUnique(ctx context.Context, phone meta.Phone) error {
	if phone.IsEmpty() {
		return nil
	}

	_, err := v.repo.FindByPhone(ctx, phone)
	if err == nil {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user with phone(%s) already exists", phone.String())
	}
	if perrors.IsCode(err, code.ErrUserNotFound) {
		return nil
	}
	return perrors.WrapC(err, code.ErrDatabase, "check user phone(%s) failed", phone.String())
}
