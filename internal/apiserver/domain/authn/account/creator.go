package account

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ==================== 账户创建器实现 ====================

// accountCreator 账户创建器实现
type accountCreator struct {
	repo       Repository
	strategies map[AccountType]CreatorStrategy
}

var _ AccountCreator = (*accountCreator)(nil)

// NewAccountCreator 创建账户创建器
// 内部自动注册所有策略，应用层无需关心策略细节
func NewAccountCreator(repo Repository) AccountCreator {
	// 注册所有策略
	strategies := map[AccountType]CreatorStrategy{
		TypeOpera:        NewOperaCreatorStrategy(),
		TypeWcMinip:      NewWechatMinipCreatorStrategy(),
		TypeWcCom:        NewWecomCreatorStrategy(),
		TypeMockConsumer: NewMockConsumerCreatorStrategy(),
	}

	return &accountCreator{
		repo:       repo,
		strategies: strategies,
	}
}

// CreateAccount 创建账户实体（不包含持久化）
// 职责：
// 1. 选择策略并准备数据
// 2. 幂等性检查（查询已存在的账户）
// 3. 创建账户实体
// 注意：持久化由应用层负责
func (c *accountCreator) CreateAccount(ctx context.Context, input CreationInput) (*Account, *CreationParams, error) {
	// ========== 步骤1: 选择创建策略 ==========
	strategy, ok := c.strategies[input.AccountType]
	if !ok {
		return nil, nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported account type: %s", input.AccountType)
	}

	if input.AccountType != TypeOpera && !input.ScopedTenantID.IsZero() {
		return nil, nil, perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is only valid for opera accounts")
	}

	// ========== 步骤2: 准备账户创建参数 ==========
	params, err := strategy.PrepareData(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	// ========== 步骤3: 幂等性检查 ==========
	existingAccount, err := c.repo.GetByExternalIDAppId(ctx, params.ExternalID, params.AppID)
	if err != nil {
		return nil, nil, err
	}

	if existingAccount != nil {
		// 账户已存在，验证是否属于同一用户
		if existingAccount.UserID != input.UserID {
			return nil, nil, perrors.WithCode(code.ErrExternalExists, "account already belongs to another user")
		}
		return existingAccount, params, nil
	}

	// ========== 步骤4: 使用策略创建账户实体 ==========
	account, err := strategy.Create(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	// 注意：账户持久化由应用层负责
	return account, params, nil
}
