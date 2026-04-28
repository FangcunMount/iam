package idp

import idpv1 "github.com/FangcunMount/iam/api/grpc/iam/idp/v1"

// Raw 返回原始 IDP 服务客户端。
func (c *Client) Raw() idpv1.IDPServiceClient {
	return c.idpService
}
