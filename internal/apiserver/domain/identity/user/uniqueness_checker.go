package user

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// UniquenessChecker 检查 User 跨实体唯一性规则。
type UniquenessChecker struct {
	repo Repository
}

// NewUniquenessChecker 创建用户唯一性检查器。
func NewUniquenessChecker(repo Repository) *UniquenessChecker {
	return &UniquenessChecker{repo: repo}
}

// CheckPhoneUnique 检查手机号唯一性。
// 手机号为空时视为未绑定手机号，不参与唯一性检查。
func (checker *UniquenessChecker) CheckPhoneUnique(ctx context.Context, phone meta.Phone) error {
	if phone.IsEmpty() {
		return nil
	}

	existing, err := checker.repo.FindByPhone(ctx, phone)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "check user phone(%s) failed", phone.String())
	}
	if existing != nil {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user with phone(%s) already exists", phone.String())
	}
	return nil
}

// CheckPhoneChange 在手机号变更时检查唯一性。
// 清空手机号，或新手机号与旧手机号相同，则不必校验。
func (checker *UniquenessChecker) CheckPhoneChange(ctx context.Context, user *User, phone meta.Phone) error {
	if user == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "user is required")
	}
	if phone.IsEmpty() || phone.Equal(user.Phone) {
		return nil
	}

	existing, err := checker.repo.FindByPhone(ctx, phone)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "check user phone(%s) failed", phone.String())
	}
	if existing != nil && (user.ID.IsZero() || existing.ID.IsZero() || existing.ID != user.ID) {
		return perrors.WithCode(code.ErrUserAlreadyExists, "user with phone(%s) already exists", phone.String())
	}
	return nil
}
