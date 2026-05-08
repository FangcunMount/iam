package request

import (
	"encoding/json"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// LoginV2Request 是 v2 显式登录请求。
type LoginV2Request struct {
	AuthMethod    string          `json:"auth_method" binding:"required"`    // 认证方式：password | phone_otp | wechat | wecom
	DeviceID      string          `json:"device_id,omitempty"`               // 设备 ID
	MethodPayload json.RawMessage `json:"method_payload" binding:"required"` // 凭证（根据 auth_method 不同而不同）
}

// Validate 验证 v2 登录请求。
func (r *LoginV2Request) Validate() error {
	if !login.IsPublicAuthMethod(r.AuthMethod) {
		return perrors.WithCode(code.ErrUnsupportedAuthMethod, "invalid authentication method: %s", r.AuthMethod)
	}
	if len(r.MethodPayload) == 0 {
		return perrors.WithCode(code.ErrPayloadInvalid, "method_payload is required")
	}
	return nil
}

// PasswordCredentials 密码认证凭证
type PasswordCredentials struct {
	// Username 登录名：须与创建账号时写入的 auth_accounts.external_id 一致（例如配置的登录名或邮箱）
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TenantID uint64 `json:"tenant_id,omitempty"`
}

// PhoneOTPCredentials 手机号验证码凭证
type PhoneOTPCredentials struct {
	Phone   string `json:"phone" binding:"required"`    // E.164 格式
	OTPCode string `json:"otp_code" binding:"required"` // 验证码
}

// PreparePhoneOTPLoginRequest 登录预准备：请求发送手机登录短信验证码
type PreparePhoneOTPLoginRequest struct {
	Phone string `json:"phone" binding:"required"` // 支持 E.164 或国内手机号，服务端规范为 E.164
}

// Validate 校验发送登录 OTP 请求
func (r *PreparePhoneOTPLoginRequest) Validate() error {
	if strings.TrimSpace(r.Phone) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}
	return nil
}

// WeChatCredentials 微信小程序凭证
type WeChatCredentials struct {
	AppID string `json:"app_id" binding:"required"` // 微信应用ID
	Code  string `json:"code" binding:"required"`   // 微信 JS Code
}

// WeComCredentials 企业微信凭证
type WeComCredentials struct {
	CorpID   string `json:"corp_id" binding:"required"`   // 企业ID
	AuthCode string `json:"auth_code" binding:"required"` // 授权码
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Validate 验证刷新令牌请求
func (r *RefreshTokenRequest) Validate() error {
	if r.RefreshToken == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "refresh_token is required")
	}
	return nil
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	AccessToken  *string `json:"access_token,omitempty"`  // 可选，撤销访问令牌
	RefreshToken string  `json:"refresh_token,omitempty"` // 可选，撤销刷新令牌
}

// Validate 验证登出请求
func (r *LogoutRequest) Validate() error {
	if r.AccessToken == nil && r.RefreshToken == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "at least one of access_token or refresh_token is required")
	}
	return nil
}

// VerifyTokenRequest 验证令牌请求
type VerifyTokenRequest struct {
	AccessToken      string   `json:"access_token" binding:"required"`
	ExpectedIssuer   string   `json:"expected_issuer,omitempty"`
	ExpectedAudience []string `json:"expected_audience,omitempty"`
}

// Validate 验证令牌验证请求
func (r *VerifyTokenRequest) Validate() error {
	if r.AccessToken == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "access_token is required")
	}
	return nil
}

// RevokeTokenRequest 撤销访问令牌请求
type RevokeTokenRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

// Validate 验证撤销令牌请求
func (r *RevokeTokenRequest) Validate() error {
	if r.AccessToken == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "access_token is required")
	}
	return nil
}

// RevokeRefreshTokenRequest 撤销刷新令牌请求
type RevokeRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Validate 验证撤销刷新令牌请求
func (r *RevokeRefreshTokenRequest) Validate() error {
	if r.RefreshToken == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "refresh_token is required")
	}
	return nil
}
