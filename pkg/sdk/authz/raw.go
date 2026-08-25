package authz

import authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"

// Raw 返回原始 AuthorizationService gRPC 客户端。
func (c *Client) Raw() authzv3.AuthorizationServiceClient {
	return c.authorizationService
}
