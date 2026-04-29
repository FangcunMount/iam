package assembler

import (
	"context"
	"fmt"

	wechatappDomain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/apiserver/infra/wechatapi"
)

type appTokenProviderAdapter struct {
	tokenProvider *wechatapi.TokenProvider
	wechatAppRepo wechatappDomain.Repository
}

func (a *appTokenProviderAdapter) Fetch(
	ctx context.Context,
	app *wechatappDomain.WechatApp,
) (*wechatappDomain.AppAccessToken, error) {
	if app.Cred == nil || app.Cred.Auth == nil {
		return nil, fmt.Errorf("app credentials not found")
	}

	return nil, fmt.Errorf("not implemented: AppTokenProvider should be called from application layer with decrypted credentials")
}
