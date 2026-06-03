package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// PrepareLink 准备微信开放平台登录身份。
func (in LinkWechatOpenInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查微信开放平台应用 ID 和认证码是否有效。
	appID := strings.TrimSpace(in.AppID)
	jsCode := strings.TrimSpace(in.Code)
	if appID == "" || jsCode == "" {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "app_id and code are required")
	}

	// 检查微信开放平台身份提供者是否配置。
	if deps.idp == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	// 解析微信开放平台应用密钥，并强校验应用类型为开放平台网站应用。
	appSecret, err := deps.resolveWechatAppSecretTyped(ctx, appID, idpPort.OpenPlatformWebsite)
	if err != nil {
		return preparedLink{}, err
	}

	// 交换微信开放平台认证码。
	openID, unionID, err := deps.idp.ExchangeWxOpenCode(ctx, appID, appSecret, jsCode)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "failed to exchange wechat code: %v", err)
	}

	// 构建提供者密钥。
	key := loginidentity.WechatOpenProviderKey(appID, openID, unionID)

	// 构建已验证登录身份。
	verifiedAt := deps.currentTime()
	return preparedLink{
		key:                     key,
		build:                   verifiedIdentityBuild(userID, key, verifiedAt),
		requireGlobalUniqueness: true,
	}, nil
}
