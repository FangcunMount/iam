// Package client 提供认证相关 gRPC 客户端能力。
package client

import (
	"context"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// Client 认证服务客户端。
//
// 提供认证相关功能，包括：
//   - Token 验证和管理（VerifyToken、RefreshToken、RevokeToken、RevokeRefreshToken）
//   - 账号开通（CreateOperationAccount）
//   - 服务间认证（IssueServiceToken）
//   - JWKS 管理（GetJWKS）
type Client struct {
	authService              authnv1.AuthServiceClient
	accountOnboardingService authnv1.AccountOnboardingServiceClient
	jwksService              authnv1.JWKSServiceClient
}

// NewClient 创建认证服务客户端。
func NewClient(
	authService authnv1.AuthServiceClient,
	accountOnboardingService authnv1.AccountOnboardingServiceClient,
	jwksService authnv1.JWKSServiceClient,
) *Client {
	return &Client{
		authService:              authService,
		accountOnboardingService: accountOnboardingService,
		jwksService:              jwksService,
	}
}

// VerifyToken 在线验证 Access Token。
func (c *Client) VerifyToken(ctx context.Context, req *authnv1.VerifyTokenRequest) (*authnv1.VerifyTokenResponse, error) {
	resp, err := c.authService.VerifyToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// CreateOperationAccount 创建运营后台账号，并按需创建用户、账户和密码凭据。
func (c *Client) CreateOperationAccount(ctx context.Context, req *authnv1.CreateOperationAccountRequest) (*authnv1.CreateOperationAccountResponse, error) {
	resp, err := c.accountOnboardingService.CreateOperationAccount(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RefreshToken 使用 Refresh Token 刷新获取新的 Access Token。
func (c *Client) RefreshToken(ctx context.Context, req *authnv1.RefreshTokenRequest) (*authnv1.RefreshTokenResponse, error) {
	resp, err := c.authService.RefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeToken 撤销 Access Token。
func (c *Client) RevokeToken(ctx context.Context, req *authnv1.RevokeTokenRequest) (*authnv1.RevokeTokenResponse, error) {
	resp, err := c.authService.RevokeToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeRefreshToken 撤销 Refresh Token。
func (c *Client) RevokeRefreshToken(ctx context.Context, req *authnv1.RevokeRefreshTokenRequest) (*authnv1.RevokeRefreshTokenResponse, error) {
	resp, err := c.authService.RevokeRefreshToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// IssueServiceToken 签发服务间认证 Token。
func (c *Client) IssueServiceToken(ctx context.Context, req *authnv1.IssueServiceTokenRequest) (*authnv1.IssueServiceTokenResponse, error) {
	resp, err := c.authService.IssueServiceToken(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetJWKS 获取 JSON Web Key Set (JWKS)。
func (c *Client) GetJWKS(ctx context.Context, req *authnv1.GetJWKSRequest) (*authnv1.GetJWKSResponse, error) {
	resp, err := c.jwksService.GetJWKS(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Raw 返回原始认证服务 gRPC 客户端。
func (c *Client) Raw() authnv1.AuthServiceClient {
	return c.authService
}

// AccountOnboardingRaw 返回原始账号开通 gRPC 客户端。
func (c *Client) AccountOnboardingRaw() authnv1.AccountOnboardingServiceClient {
	return c.accountOnboardingService
}

// JWKSRaw 返回原始 JWKS 服务 gRPC 客户端。
func (c *Client) JWKSRaw() authnv1.JWKSServiceClient {
	return c.jwksService
}
