// Package response 定义 IDP 模块 REST API 响应结构
package response

// ============= 微信应用管理响应 =============

// WechatAppResponse 微信应用响应
type WechatAppResponse struct {
	ID     string `json:"id"`     // 内部 ID
	AppID  string `json:"app_id"` // 微信应用 ID
	Name   string `json:"name"`   // 应用名称
	Type   string `json:"type"`   // 应用类型（MiniProgram/MP）
	Status string `json:"status"` // 应用状态（Enabled/Disabled/Archived）
}

// WechatAppListResponse 微信应用列表响应。
type WechatAppListResponse struct {
	Total int                  `json:"total"`
	Items []*WechatAppResponse `json:"items"`
}

// AccessTokenResponse 访问令牌响应
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"` // 访问令牌
	ExpiresIn   int    `json:"expires_in"`   // 过期时间（秒）
}

// RotateSecretResponse 轮换密钥响应
type RotateSecretResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// ============= 微信认证响应 =============

// LoginResponse 微信登录响应
type LoginResponse struct {
	// 外部身份声明
	Provider    string  `json:"provider"`               // 身份提供商（wechat_miniprogram）
	AppID       string  `json:"app_id"`                 // 微信应用 ID
	OpenID      string  `json:"open_id"`                // 用户 OpenID
	UnionID     *string `json:"union_id,omitempty"`     // 用户 UnionID（可选）
	DisplayName *string `json:"display_name,omitempty"` // 显示名称（可选）
	AvatarURL   *string `json:"avatar_url,omitempty"`   // 头像 URL（可选）
	Phone       *string `json:"phone,omitempty"`        // 手机号（可选）
	Email       *string `json:"email,omitempty"`        // 邮箱（可选）
	ExpiresIn   int     `json:"expires_in"`             // 过期时间（秒）

	// 会话信息
	SessionKey string `json:"session_key"` // Session Key（加密后）
	Version    int    `json:"version"`     // 会话版本
}

// DecryptPhoneResponse 解密手机号响应
type DecryptPhoneResponse struct {
	Phone string `json:"phone"` // 手机号
}

// ============= 通用响应 =============

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
}
