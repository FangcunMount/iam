// Package challenge provides a REST AuthN v2 client for public authentication challenges.
package challenge

import (
	"strings"
	"time"

	"github.com/FangcunMount/iam/v2/pkg/sdk/auth/internal/restclient"
)

type SendLoginPhoneOTPRequest struct {
	Phone string `json:"phone"`
}

func (r SendLoginPhoneOTPRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return restclient.InvalidArgument("phone is required")
	}
	return nil
}

type MessageResponse struct {
	Message string `json:"message"`
}

// WechatOpenAuthorizeRequest starts a WeChat Open Platform website QR login authorization.
type WechatOpenAuthorizeRequest struct {
	Nonce string `json:"nonce,omitempty"`
}

func (WechatOpenAuthorizeRequest) Validate() error {
	return nil
}

// WechatOpenAuthorizeResponse contains the OAuth state and redirect URL for WeChat QR login.
type WechatOpenAuthorizeResponse struct {
	State        string    `json:"state"`
	Nonce        string    `json:"nonce"`
	AppID        string    `json:"app_id"`
	AuthorizeURL string    `json:"authorize_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}
