package response

import "time"

type LoginIdentityResponse struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	Realm            string     `json:"realm"`
	Identifier       string     `json:"identifier"`
	GlobalIdentifier string     `json:"global_identifier,omitempty"`
	Status           string     `json:"status"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	LinkedAt         time.Time  `json:"linked_at"`
}

type LoginIdentityListResponse struct {
	Items []LoginIdentityResponse `json:"items"`
}

type LinkLoginIdentityResponse struct {
	LoginIdentity LoginIdentityResponse `json:"login_identity"`
	Reused        bool                  `json:"reused"`
}

// WechatOpenAuthorizeResponse 返回微信开放平台扫码授权地址与 state（登录/绑定通用）。
//
// AppID 用于登录回调时前端回传 /authn/login（绑定流程不需要，但一并回显无害）。
type WechatOpenAuthorizeResponse struct {
	State        string    `json:"state"`
	Nonce        string    `json:"nonce,omitempty"`
	AppID        string    `json:"app_id,omitempty"`
	AuthorizeURL string    `json:"authorize_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}
