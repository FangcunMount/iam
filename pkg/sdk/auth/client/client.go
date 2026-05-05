// Package client 提供认证相关 gRPC 客户端能力。
package client

import (
	"context"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// Client 认证服务客户端。
//
// 提供认证相关功能，包括：
//   - Token 验证和管理（VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken）
//   - 账号开通（CreateOperationAccount）
//   - 服务间认证（IssueServiceToken）
//   - JWKS 管理（GetJWKS）
type Client struct {
	authService              authnv2.AuthServiceClient
	accountOnboardingService authnv2.AccountOnboardingServiceClient
	jwksService              authnv2.JWKSServiceClient
}

// NewClient 创建认证服务客户端。
func NewClient(
	authService authnv2.AuthServiceClient,
	accountOnboardingService authnv2.AccountOnboardingServiceClient,
	jwksService authnv2.JWKSServiceClient,
) *Client {
	return &Client{
		authService:              authService,
		accountOnboardingService: accountOnboardingService,
		jwksService:              jwksService,
	}
}

// VerifyToken 在线验证 Access Token。
func (c *Client) VerifyToken(ctx context.Context, req *authnv2.VerifyTokenRequest) (*authnv2.VerifyTokenResponse, error) {
	resp, err := c.authService.VerifyToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Login 使用 v2 explicit auth_method + method_payload 契约登录。
func (c *Client) Login(ctx context.Context, req *authnv2.LoginRequest) (*authnv2.LoginResponse, error) {
	resp, err := c.authService.Login(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// CreateOperationAccount 创建运营后台账号，并按需创建用户、账户和密码凭据。
func (c *Client) CreateOperationAccount(ctx context.Context, req *authnv2.CreateOperationAccountRequest) (*authnv2.CreateOperationAccountResponse, error) {
	resp, err := c.accountOnboardingService.CreateOperationAccount(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RefreshToken 使用 Refresh Token 刷新获取新的 Access Token。
func (c *Client) RefreshToken(ctx context.Context, req *authnv2.RefreshTokenRequest) (*authnv2.RefreshTokenResponse, error) {
	resp, err := c.authService.RefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeToken 撤销 Access Token。
func (c *Client) RevokeToken(ctx context.Context, req *authnv2.RevokeTokenRequest) (*authnv2.RevokeTokenResponse, error) {
	resp, err := c.authService.RevokeToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeRefreshToken 撤销 Refresh Token。
func (c *Client) RevokeRefreshToken(ctx context.Context, req *authnv2.RevokeRefreshTokenRequest) (*authnv2.RevokeRefreshTokenResponse, error) {
	resp, err := c.authService.RevokeRefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// IssueServiceToken 签发服务间认证 Token。
func (c *Client) IssueServiceToken(ctx context.Context, req *authnv2.IssueServiceTokenRequest) (*authnv2.IssueServiceTokenResponse, error) {
	resp, err := c.authService.IssueServiceToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetJWKS 获取 JSON Web Key Set (JWKS)。
func (c *Client) GetJWKS(ctx context.Context, req *authnv2.GetJWKSRequest) (*authnv2.GetJWKSResponse, error) {
	resp, err := c.jwksService.GetJWKS(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Raw 返回原始认证服务 gRPC 客户端。
func (c *Client) Raw() authnv2.AuthServiceClient {
	return c.authService
}

// AccountOnboardingRaw 返回原始账号开通 gRPC 客户端。
func (c *Client) AccountOnboardingRaw() authnv2.AccountOnboardingServiceClient {
	return c.accountOnboardingService
}

// JWKSRaw 返回原始 JWKS 服务 gRPC 客户端。
func (c *Client) JWKSRaw() authnv2.JWKSServiceClient {
	return c.jwksService
}
