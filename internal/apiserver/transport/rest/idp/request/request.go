// Package request 定义 IDP 模块 REST API 请求结构
package request

// ListWechatAppsRequest 微信应用列表请求（Query 参数）。
type ListWechatAppsRequest struct {
	Type   string `form:"type"`
	Status string `form:"status"`
}

// ============= 微信应用管理请求 =============

// CreateWechatAppRequest 创建微信应用请求
type CreateWechatAppRequest struct {
	AppID     string `json:"app_id" binding:"required"`      // 微信应用 ID（必填）
	Name      string `json:"name" binding:"required"`        // 应用名称（必填）
	Type      string `json:"type" binding:"required"`        // 应用类型（MiniProgram/MP，必填）
	AppSecret string `json:"app_secret" binding:"omitempty"` // AppSecret（可选，创建时设置）
}

// GetWechatAppRequest 查询微信应用请求（URI 参数）
type GetWechatAppRequest struct {
	AppID string `uri:"app_id" binding:"required"` // 微信应用 ID
}

// UpdateWechatAppRequest 更新微信应用基础信息请求。
type UpdateWechatAppRequest struct {
	Name *string `json:"name"`
	Type *string `json:"type"`
}

// RotateAuthSecretRequest 轮换认证密钥请求
type RotateAuthSecretRequest struct {
	AppID     string `json:"app_id" binding:"required"`     // 微信应用 ID
	NewSecret string `json:"new_secret" binding:"required"` // 新的 AppSecret
}

// RotateMsgSecretRequest 轮换消息密钥请求
type RotateMsgSecretRequest struct {
	AppID          string `json:"app_id" binding:"required"`           // 微信应用 ID
	CallbackToken  string `json:"callback_token" binding:"required"`   // 回调 Token
	EncodingAESKey string `json:"encoding_aes_key" binding:"required"` // 加密 AES Key
}

// GetAccessTokenRequest 获取访问令牌请求（URI 参数）
type GetAccessTokenRequest struct {
	AppID string `uri:"app_id" binding:"required"` // 微信应用 ID
}

// RefreshAccessTokenRequest 刷新访问令牌请求
type RefreshAccessTokenRequest struct {
	AppID string `json:"app_id" binding:"required"` // 微信应用 ID
}

// ============= 微信认证请求 =============

// LoginWithCodeRequest 微信登录请求
type LoginWithCodeRequest struct {
	AppID  string `json:"app_id" binding:"required"`  // 微信应用 ID
	JSCode string `json:"js_code" binding:"required"` // 微信登录码（小程序 wx.login 获取）
}

// DecryptPhoneRequest 解密手机号请求
type DecryptPhoneRequest struct {
	AppID         string `json:"app_id" binding:"required"`         // 微信应用 ID
	OpenID        string `json:"open_id" binding:"required"`        // 用户 OpenID
	EncryptedData string `json:"encrypted_data" binding:"required"` // 加密数据
	IV            string `json:"iv" binding:"required"`             // 加密算法的初始向量
}
