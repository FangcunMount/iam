// Package client 提供认证相关 gRPC 客户端能力。
package client

import (
	"context"

	authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
	"github.com/FangcunMount/iam/v5/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client 认证服务客户端。
//
// 提供认证相关功能，包括：
//   - Token 验证和管理（VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken）
//   - 生产 AuthN 契约（Signup、Challenge、LoginIdentity）
//   - JWKS 管理（GetJWKS）
type Client struct {
	authService                  authnv3.AuthServiceClient
	authSignupService            authnv3.AuthSignupServiceClient
	authChallengeService         authnv3.AuthChallengeServiceClient
	loginIdentityService         authnv3.LoginIdentityServiceClient
	notificationRecipientService authnv3.NotificationRecipientServiceClient
	jwksService                  authnv3.JWKSServiceClient
}

// NewClient 创建认证服务客户端。
func NewClient(
	authService authnv3.AuthServiceClient,
	jwksService authnv3.JWKSServiceClient,
	optionalServices ...any,
) *Client {
	c := &Client{
		authService: authService,
		jwksService: jwksService,
	}
	for _, service := range optionalServices {
		switch typed := service.(type) {
		case authnv3.AuthSignupServiceClient:
			c.authSignupService = typed
		case authnv3.AuthChallengeServiceClient:
			c.authChallengeService = typed
		case authnv3.LoginIdentityServiceClient:
			c.loginIdentityService = typed
		case authnv3.NotificationRecipientServiceClient:
			c.notificationRecipientService = typed
		}
	}
	return c
}

// ResolveMiniProgramRecipients reads only the AppID-scoped recipient identities
// exposed to an authorized service client; callers must avoid logging OpenIDs.
func (c *Client) ResolveMiniProgramRecipients(ctx context.Context, req *authnv3.ResolveMiniProgramNotificationRecipientsRequest) (*authnv3.ResolveMiniProgramNotificationRecipientsResponse, error) {
	if c == nil || c.notificationRecipientService == nil {
		return nil, status.Error(codes.Unavailable, "notification recipient service is unavailable")
	}
	resp, err := c.notificationRecipientService.ResolveMiniProgramRecipients(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// VerifyToken 在线验证 Access Token。
func (c *Client) VerifyToken(ctx context.Context, req *authnv3.VerifyTokenRequest) (*authnv3.VerifyTokenResponse, error) {
	resp, err := c.authService.VerifyToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Login 使用 v2 explicit auth_method + method_payload 契约登录。
func (c *Client) Login(ctx context.Context, req *authnv3.LoginRequest) (*authnv3.LoginResponse, error) {
	resp, err := c.authService.Login(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RefreshToken 使用 Refresh Token 刷新获取新的 Access Token。
func (c *Client) RefreshToken(ctx context.Context, req *authnv3.RefreshTokenRequest) (*authnv3.RefreshTokenResponse, error) {
	resp, err := c.authService.RefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeToken 撤销 Access Token。
func (c *Client) RevokeToken(ctx context.Context, req *authnv3.RevokeTokenRequest) (*authnv3.RevokeTokenResponse, error) {
	resp, err := c.authService.RevokeToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeRefreshToken 撤销 Refresh Token。
func (c *Client) RevokeRefreshToken(ctx context.Context, req *authnv3.RevokeRefreshTokenRequest) (*authnv3.RevokeRefreshTokenResponse, error) {
	resp, err := c.authService.RevokeRefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// SignUpWithWechatMiniProgram 通过微信小程序开通 User + LoginIdentity。
func (c *Client) SignUpWithWechatMiniProgram(ctx context.Context, req *authnv3.SignUpWithWechatMiniProgramRequest) (*authnv3.SignupResult, error) {
	if c.authSignupService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "auth signup service client not configured"))
	}
	resp, err := c.authSignupService.SignUpWithWechatMiniProgram(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// SendLoginPhoneOTP 发送手机号登录验证码。
func (c *Client) SendLoginPhoneOTP(ctx context.Context, req *authnv3.SendLoginPhoneOTPRequest) (*authnv3.MessageResponse, error) {
	if c.authChallengeService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "auth challenge service client not configured"))
	}
	resp, err := c.authChallengeService.SendLoginPhoneOTP(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListLoginIdentities 列出已绑定登录身份。
func (c *Client) ListLoginIdentities(ctx context.Context, req *authnv3.ListLoginIdentitiesRequest) (*authnv3.ListLoginIdentitiesResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.ListLoginIdentities(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// SendPhoneLinkChallenge 发送手机号绑定验证码。
func (c *Client) SendPhoneLinkChallenge(ctx context.Context, req *authnv3.SendPhoneLinkChallengeRequest) (*authnv3.MessageResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.SendPhoneLinkChallenge(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// LinkPhone 绑定手机号登录身份。
func (c *Client) LinkPhone(ctx context.Context, req *authnv3.LinkPhoneRequest) (*authnv3.LinkLoginIdentityResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.LinkPhone(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// LinkWechatMiniProgram 绑定微信小程序登录身份。
func (c *Client) LinkWechatMiniProgram(ctx context.Context, req *authnv3.LinkWechatMiniProgramRequest) (*authnv3.LinkLoginIdentityResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.LinkWechatMiniProgram(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// LinkWecom 绑定企业微信登录身份。
func (c *Client) LinkWecom(ctx context.Context, req *authnv3.LinkWecomRequest) (*authnv3.LinkLoginIdentityResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.LinkWecom(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// UnlinkLoginIdentity 解绑登录身份。
func (c *Client) UnlinkLoginIdentity(ctx context.Context, req *authnv3.UnlinkLoginIdentityRequest) (*authnv3.MessageResponse, error) {
	if c.loginIdentityService == nil {
		return nil, errors.Wrap(status.Error(codes.Unimplemented, "login identity service client not configured"))
	}
	resp, err := c.loginIdentityService.UnlinkLoginIdentity(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetJWKS 获取 JSON Web Key Set (JWKS)。
func (c *Client) GetJWKS(ctx context.Context, req *authnv3.GetJWKSRequest) (*authnv3.GetJWKSResponse, error) {
	resp, err := c.jwksService.GetJWKS(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Raw 返回原始认证服务 gRPC 客户端。
func (c *Client) Raw() authnv3.AuthServiceClient {
	return c.authService
}

// JWKSRaw 返回原始 JWKS 服务 gRPC 客户端。
func (c *Client) JWKSRaw() authnv3.JWKSServiceClient {
	return c.jwksService
}

// SignupRaw 返回原始 signup gRPC 客户端。
func (c *Client) SignupRaw() authnv3.AuthSignupServiceClient {
	return c.authSignupService
}

// ChallengeRaw 返回原始 challenge gRPC 客户端。
func (c *Client) ChallengeRaw() authnv3.AuthChallengeServiceClient {
	return c.authChallengeService
}

// LoginIdentityRaw 返回原始 login identity gRPC 客户端。
func (c *Client) LoginIdentityRaw() authnv3.LoginIdentityServiceClient {
	return c.loginIdentityService
}
