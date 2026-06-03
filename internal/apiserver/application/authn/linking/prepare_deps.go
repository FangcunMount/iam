package linking

import (
	"context"
	"time"

	idpprepare "github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/prepare"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

// linkPrepareDeps 是 prepare 阶段可用的依赖快照，避免各 Input 依赖 *linker。
type linkPrepareDeps struct {
	phoneLinkOTP PhoneLinkChallengeVerifier
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
	provider := idpprepare.ProviderWechat
	if providerName == "wecom" {
		provider = idpprepare.ProviderWecom
	}
	return idpprepare.ResolveAppSecret(ctx, idpprepare.Dependencies{
		Apps:  d.wechatApps,
		Vault: d.secretVault,
	}, idpprepare.Options{
		Provider: provider,
		Surface:  idpprepare.SurfaceLinking,
		AppID:    appID,
	})
}

// resolveWechatAppSecretTyped 解析微信应用密钥并强校验应用类型。
func (d linkPrepareDeps) resolveWechatAppSecretTyped(ctx context.Context, appID string, expectedType idpPort.AppType) (string, error) {
	return idpprepare.ResolveAppSecret(ctx, idpprepare.Dependencies{
		Apps:  d.wechatApps,
		Vault: d.secretVault,
	}, idpprepare.Options{
		Provider:        idpprepare.ProviderWechat,
		Surface:         idpprepare.SurfaceLinking,
		AppID:           appID,
		ExpectedAppType: expectedType,
	})
}
