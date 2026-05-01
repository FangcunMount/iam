package account

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ==================== 微信小程序账户创建策略 ====================

// WechatMinipCreatorStrategy 微信小程序账户创建策略（TypeWcMinip）
type WechatMinipCreatorStrategy struct{}

var _ CreatorStrategy = (*WechatMinipCreatorStrategy)(nil)

// NewWechatMinipCreatorStrategy 创建微信小程序创建策略
func NewWechatMinipCreatorStrategy() *WechatMinipCreatorStrategy {
	return &WechatMinipCreatorStrategy{}
}

// Kind 返回策略支持的账户类型
func (s *WechatMinipCreatorStrategy) Kind() AccountType {
	return TypeWcMinip
}

// PrepareData 准备微信小程序账户创建参数
// 微信 code2session 属于应用层身份解析；领域层只接受已解析的 OpenID/UnionID。
func (s *WechatMinipCreatorStrategy) PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error) {
	// 验证必要参数
	if input.WechatAppID == nil || *input.WechatAppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appid is required for wechat minip account")
	}
	if input.WechatOpenID == nil || *input.WechatOpenID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat openid is required for wechat minip account")
	}

	openID := *input.WechatOpenID
	unionID := ""
	if input.WechatUnionID != nil {
		unionID = *input.WechatUnionID
	}

	// 构造 ExternalID：OpenID@AppID
	externalID := ExternalID(fmt.Sprintf("%s@%s", openID, *input.WechatAppID))
	appID := AppId(*input.WechatAppID)

	return &CreationParams{
		UserID:      input.UserID,
		AccountType: TypeWcMinip,
		AppID:       appID,
		ExternalID:  externalID,
		OpenID:      openID,
		UnionID:     unionID,
		Profile:     input.Profile,
		Meta:        input.Meta,
		ParamsJSON:  input.ParamsJSON,
	}, nil
}

// Create 创建微信小程序账户实体
func (s *WechatMinipCreatorStrategy) Create(ctx context.Context, params *CreationParams) (*Account, error) {
	// 创建账户实体
	account := NewAccount(
		params.UserID,
		TypeWcMinip,
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
