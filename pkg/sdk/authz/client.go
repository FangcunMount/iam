// Package authz 提供授权判定（PDP）能力。
package authz

import authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"

// Client 授权服务客户端。
type Client struct {
	authorizationService authzv3.AuthorizationServiceClient
}

// NewClient 创建授权服务客户端。
func NewClient(authorizationService authzv3.AuthorizationServiceClient) *Client {
	return &Client{
		authorizationService: authorizationService,
	}
}
