package authz

import authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"

// Raw 返回原始 AuthorizationService gRPC 客户端。
func (c *Client) Raw() authzv2.AuthorizationServiceClient {
	return c.authorizationService
}
