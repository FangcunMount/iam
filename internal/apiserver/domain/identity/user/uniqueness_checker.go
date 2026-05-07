package user

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// UniquenessChecker 检查 User 跨实体唯一性规则。
type UniquenessChecker struct {
	repo Repository
}

// NewUniquenessChecker 创建用户唯一性检查器。
func NewUniquenessChecker(repo Repository) *UniquenessChecker {
	return &UniquenessChecker{repo: repo}
}

// CheckPhoneChange 在手机号变更时检查唯一性。
func (c *UniquenessChecker) CheckPhoneChange(ctx context.Context, user *User, phone meta.Phone) error {
	// 如果手机号变更，检查唯一性
	if !phone.IsEmpty() && !user.Phone.Equal(phone) {
		if err := c.CheckPhoneUnique(ctx, phone); err != nil {
			return err
		}
	}
	return nil
}

// CheckPhoneUnique 检查手机号唯一性
func (c *UniquenessChecker) CheckPhoneUnique(ctx context.Context, phone meta.Phone) error {
	if phone.IsEmpty() {
		return nil
	}

	_, err := c.repo.FindByPhone(ctx, phone)
	if err == nil {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user with phone(%s) already exists", phone.String())
	}
	if perrors.IsCode(err, code.ErrUserNotFound) {
		return nil
	}
	return perrors.WrapC(err, code.ErrDatabase, "check user phone(%s) failed", phone.String())
}
