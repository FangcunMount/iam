// Package idp 提供身份提供者（IDP）能力。
package idp

import idpv1 "github.com/FangcunMount/iam/api/grpc/iam/idp/v1"

// Client IDP 服务客户端。
type Client struct {
	idpService idpv1.IDPServiceClient
}

// NewClient 创建 IDP 服务客户端。
func NewClient(idp idpv1.IDPServiceClient) *Client {
	return &Client{
		idpService: idp,
	}
}
