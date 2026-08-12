package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// PrepareLink 准备微信小程序登录身份。
func (in LinkWechatMiniInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查微信小程序应用 ID 和认证码是否有效。
	appID := strings.TrimSpace(in.AppID)
	jsCode := strings.TrimSpace(in.Code)
	if appID == "" || jsCode == "" {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "app_id and code are required")
	}
	// 检查微信小程序身份提供者是否配置。
	if deps.idp == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	// 解析微信小程序应用密钥。
	appSecret, err := deps.resolveAppSecret(ctx, appID, "wechat")
	if err != nil {
		return preparedLink{}, err
	}

	// 交换微信小程序认证码。
	openID, unionID, err := deps.idp.ExchangeWxMinipCode(ctx, appID, appSecret, jsCode)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "failed to exchange wechat code: %v", err)
	}

	// 构建提供者密钥。
	key := loginidentity.WechatMinipProviderKey(appID, openID, unionID)

	// 构建已验证登录身份。
	verifiedAt := deps.currentTime()
	return preparedLink{
		key:                     key,
		build:                   verifiedIdentityBuild(userID, key, verifiedAt),
		requireGlobalUniqueness: true,
	}, nil
}
