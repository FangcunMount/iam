package authn

import "github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi"

type wechatOpenAuthorizeURLBuilder struct{}

func (wechatOpenAuthorizeURLBuilder) BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error) {
	return wechatapi.BuildWebAppQRConnectURL(appID, redirectURI, state)
}
