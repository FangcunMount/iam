package profile

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// IDCardUniquenessChecker 检查 Profile 身份证跨实体唯一性规则。
type IDCardUniquenessChecker struct {
	repo Repository
}

// 确保 IDCardUniquenessChecker 实现了 IDCardUniquenessChecking 接口
var _ IDCardUniquenessChecking = &IDCardUniquenessChecker{}

// NewIDCardUniquenessChecker 创建档案身份证唯一性检查器。
func NewIDCardUniquenessChecker(repo Repository) *IDCardUniquenessChecker {
	return &IDCardUniquenessChecker{repo: repo}
}

// CheckIDCardUnique 检查身份证是否还没有被其他 Profile 使用。
func (c *IDCardUniquenessChecker) CheckIDCardUnique(ctx context.Context, idCard meta.IDCard) error {
	if idCard.String() == "" {
		return nil
	}

	existing, err := c.findByIDCard(ctx, idCard)
	if err != nil {
		return err
	}
	if existing != nil {
		return perrors.WithCode(code.ErrIdentityProfileExists, "profile with id card(%s) already exists", idCard.String())
	}
	return nil
}

func (c *IDCardUniquenessChecker) findByIDCard(ctx context.Context, idCard meta.IDCard) (*Profile, error) {
	existing, err := c.repo.FindByIDCard(ctx, idCard)
	if err == nil {
		return existing, nil
	}
	if perrors.IsCode(err, code.ErrIdentityProfileNotFound) {
		return nil, nil
	}
	return nil, perrors.WrapC(err, code.ErrDatabase, "check profile id card(%s) failed", idCard.String())
}
