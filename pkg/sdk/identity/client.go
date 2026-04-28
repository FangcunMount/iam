// Package identity 提供身份管理和监护关系查询能力。
package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// Client 身份服务客户端。
type Client struct {
	readService      identityv1.IdentityReadClient
	lifecycleService identityv1.IdentityLifecycleClient
}

// NewClient 创建身份服务客户端。
func NewClient(read identityv1.IdentityReadClient, lifecycle identityv1.IdentityLifecycleClient) *Client {
	return &Client{
		readService:      read,
		lifecycleService: lifecycle,
	}
}
