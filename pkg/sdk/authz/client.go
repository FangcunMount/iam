// Package authz 提供授权判定（PDP）能力。
package authz

import authzv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v2"

// Client 授权服务客户端。
type Client struct {
	authorizationService authzv2.AuthorizationServiceClient
}

// NewClient 创建授权服务客户端。
func NewClient(authorizationService authzv2.AuthorizationServiceClient) *Client {
	return &Client{
		authorizationService: authorizationService,
	}
}
