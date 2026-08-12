package idp

import (
	"context"
	"fmt"

	wechatappDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi"
)

type wechatAccessTokenProvider interface {
	FetchMiniProgramToken(ctx context.Context, appID, appSecret string) (*wechatapi.AccessTokenResult, error)
	FetchOfficialAccountToken(ctx context.Context, appID, appSecret string) (*wechatapi.AccessTokenResult, error)
}

type appTokenProviderAdapter struct {
	tokenProvider wechatAccessTokenProvider
	secretVault   wechatappDomain.SecretVault
}

func (a *appTokenProviderAdapter) Fetch(
	ctx context.Context,
	app *wechatappDomain.WechatApp,
) (*wechatappDomain.AppAccessToken, error) {
	if app == nil {
		return nil, fmt.Errorf("wechat app is required")
	}
	if app.Cred == nil || app.Cred.Auth == nil {
		return nil, fmt.Errorf("app credentials not found")
	}
	if app.AppID == "" {
		return nil, fmt.Errorf("wechat app id is required")
	}
	if len(app.Cred.Auth.AppSecretCipher) == 0 {
		return nil, fmt.Errorf("app secret cipher not found")
	}
	if a.secretVault == nil {
		return nil, fmt.Errorf("secret vault is required")
	}
	if a.tokenProvider == nil {
		return nil, fmt.Errorf("wechat token provider is required")
	}

	secretBytes, err := a.secretVault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt app secret: %w", err)
	}
	defer zeroBytes(secretBytes)

	var result *wechatapi.AccessTokenResult
	switch app.Type {
	case wechatappDomain.MiniProgram:
		result, err = a.tokenProvider.FetchMiniProgramToken(ctx, app.AppID, string(secretBytes))
	case wechatappDomain.MP:
		result, err = a.tokenProvider.FetchOfficialAccountToken(ctx, app.AppID, string(secretBytes))
	default:
		return nil, fmt.Errorf("unsupported wechat app type: %s", app.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wechat access token: %w", err)
	}
	if result == nil || result.Token == "" {
		return nil, fmt.Errorf("wechat access token provider returned empty token")
	}

	return &wechatappDomain.AppAccessToken{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
	}, nil
}

func zeroBytes(values []byte) {
	for i := range values {
		values[i] = 0
	}
}
