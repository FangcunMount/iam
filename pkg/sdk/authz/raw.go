package authz

import authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"

// Raw 返回原始 AuthorizationService gRPC 客户端。
func (c *Client) Raw() authzv1.AuthorizationServiceClient {
	return c.authorizationService
}
