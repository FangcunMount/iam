package idp

import idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"

// Raw 返回原始 IDP 服务客户端。
func (c *Client) Raw() idpv2.IDPServiceClient {
	return c.idpService
}
