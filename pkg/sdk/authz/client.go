// Package authz 提供授权判定（PDP）能力。
package authz

import authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"

// Client 授权服务客户端。
type Client struct {
	authorizationService authzv1.AuthorizationServiceClient
}

// NewClient 创建授权服务客户端。
func NewClient(authorizationService authzv1.AuthorizationServiceClient) *Client {
	return &Client{
		authorizationService: authorizationService,
	}
}
