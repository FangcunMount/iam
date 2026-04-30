package account

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== 企业微信账户创建策略 ====================

// WecomCreatorStrategy 企业微信账户创建策略（TypeWcCom）
type WecomCreatorStrategy struct{}

var _ CreatorStrategy = (*WecomCreatorStrategy)(nil)

// NewWecomCreatorStrategy 创建企业微信创建策略
func NewWecomCreatorStrategy() *WecomCreatorStrategy {
	return &WecomCreatorStrategy{}
}

// Kind 返回策略支持的账户类型
func (s *WecomCreatorStrategy) Kind() AccountType {
	return TypeWcCom
}

// PrepareData 准备企业微信账户创建参数
func (s *WecomCreatorStrategy) PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error) {
	// 验证必要参数
	if input.WecomCorpID == nil || *input.WecomCorpID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom corpid is required for wecom account")
	}
	if input.WecomUserID == nil || *input.WecomUserID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom userid is required for wecom account")
	}

	// 企业微信账户使用 UserID 作为 ExternalID
	externalID := ExternalID(*input.WecomUserID)
	appID := AppId(*input.WecomCorpID)

	return &CreationParams{
		UserID:      input.UserID,
		AccountType: TypeWcCom,
		AppID:       appID,
		ExternalID:  externalID,
		Profile:     input.Profile,
		Meta:        input.Meta,
		ParamsJSON:  input.ParamsJSON,
	}, nil
}

// Create 创建企业微信账户实体
func (s *WecomCreatorStrategy) Create(ctx context.Context, params *CreationParams) (*Account, error) {
	// 创建账户实体
	account := NewAccount(
		params.UserID,
		TypeWcCom,
		params.ExternalID,
		WithAppID(params.AppID),
	)

	// 设置资料和元数据
	if len(params.Profile) > 0 {
		account.Profile = params.Profile
	}
	if len(params.Meta) > 0 {
		account.Meta = params.Meta
	}

	return account, nil
}
