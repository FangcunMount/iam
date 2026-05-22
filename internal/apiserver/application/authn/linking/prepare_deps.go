package linking

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// linkPrepareDeps 是 prepare 阶段可用的依赖快照，避免各 Input 依赖 *linker。
type linkPrepareDeps struct {
	challenge    challengeapp.Service
	idp          authentication.IdentityProvider
	wechatApps   idpPort.Repository
	secretVault  idpPort.SecretVault
	wecomAgentID string
	now          func() time.Time
}

// currentTime 获取当前时间。
func (d linkPrepareDeps) currentTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// resolveAppSecret 解析应用密钥。
func (d linkPrepareDeps) resolveAppSecret(ctx context.Context, appID, providerName string) (string, error) {
	// 检查微信小程序应用仓库和密钥仓库是否配置。
	if d.wechatApps == nil || d.secretVault == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "%s app configuration service is not available", providerName)
	}

	// 查询微信小程序应用。
	app, err := d.wechatApps.GetByAppID(ctx, appID)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query %s app: %v", providerName, err)
	}

	// 检查微信小程序应用是否启用。
	if app == nil || !app.IsEnabled() || app.Cred == nil || app.Cred.Auth == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "%s app is not available", providerName)
	}

	// 解密微信小程序应用密钥。
	plain, err := d.secretVault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt %s app secret: %v", providerName, err)
	}

	// 返回解密后的应用密钥。
	return string(plain), nil
}
