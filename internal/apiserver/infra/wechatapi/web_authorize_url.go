package wechatapi

import (
	"fmt"
	"net/url"
	"strings"
)

const wechatWebAppQRConnectScope = "snsapi_login"

// BuildWebAppQRConnectURL 构造网站应用微信扫码登录授权地址（qrconnect）。
// 文档: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
func BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error) {
	appID = strings.TrimSpace(appID)
	redirectURI = strings.TrimSpace(redirectURI)
	state = strings.TrimSpace(state)
	if appID == "" {
		return "", fmt.Errorf("appID is required")
	}
	if redirectURI == "" {
		return "", fmt.Errorf("redirectURI is required")
	}
	if state == "" {
		return "", fmt.Errorf("state is required")
	}

	values := url.Values{}
	values.Set("appid", appID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", wechatWebAppQRConnectScope)
	values.Set("state", state)

	return "https://open.weixin.qq.com/connect/qrconnect?" + values.Encode() + "#wechat_redirect", nil
}
