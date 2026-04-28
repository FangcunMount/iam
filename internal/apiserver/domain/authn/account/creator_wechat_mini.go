package account

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== 微信小程序账户创建策略 ====================

// WechatMinipCreatorStrategy 微信小程序账户创建策略（TypeWcMinip）
type WechatMinipCreatorStrategy struct {
	idp authentication.IdentityProvider // 用于 code2session
}

var _ CreatorStrategy = (*WechatMinipCreatorStrategy)(nil)

// NewWechatMinipCreatorStrategy 创建微信小程序创建策略
func NewWechatMinipCreatorStrategy(idp authentication.IdentityProvider) *WechatMinipCreatorStrategy {
	return &WechatMinipCreatorStrategy{
		idp: idp,
	}
}

// Kind 返回策略支持的账户类型
func (s *WechatMinipCreatorStrategy) Kind() AccountType {
	return TypeWcMinip
}

// PrepareData 准备微信小程序账户创建参数
// 如果提供了 JsCode，则调用微信 code2session 获取 OpenID 和 UnionID
func (s *WechatMinipCreatorStrategy) PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error) {
	// 验证必要参数
	if input.WechatAppID == nil || *input.WechatAppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appid is required for wechat minip account")
	}

	var openID, unionID, sessionKey string

	// 如果提供了 JsCode，调用 code2session
	if input.WechatJsCode != nil && *input.WechatJsCode != "" {
		if input.WechatAppSecret == nil || *input.WechatAppSecret == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appsecret is required for code2session")
		}

		// 调用 IDP 的 ExchangeWxMinipCode（code2session）
		oID, uID, err := s.idp.ExchangeWxMinipCode(ctx, *input.WechatAppID, *input.WechatAppSecret, *input.WechatJsCode)
		if err != nil {
			return nil, perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", err)
		}

		openID = oID
		unionID = uID
	} else if input.WechatOpenID != nil && *input.WechatOpenID != "" {
		// 如果没有 JsCode 但有 OpenID，直接使用
		openID = *input.WechatOpenID
		if input.WechatUnionID != nil {
			unionID = *input.WechatUnionID
		}
	} else {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat jscode or openid is required")
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
		Session:     sessionKey,
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
