// Package idp 提供身份提供者（IDP）能力。
package idp

import idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"

// Client IDP 服务客户端。
type Client struct {
	idpService idpv2.IDPServiceClient
}

// NewClient 创建 IDP 服务客户端。
func NewClient(idp idpv2.IDPServiceClient) *Client {
	return &Client{
		idpService: idp,
	}
}
