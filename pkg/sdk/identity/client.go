// Package identity 提供身份管理、档案命令和档案关系能力。
package identity

import identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"

// Client 身份服务客户端。
type Client struct {
	readService      identityv2.IdentityReadClient
	lifecycleService identityv2.IdentityLifecycleClient
}

// NewClient 创建身份服务客户端。
func NewClient(read identityv2.IdentityReadClient, lifecycle identityv2.IdentityLifecycleClient) *Client {
	return &Client{
		readService:      read,
		lifecycleService: lifecycle,
	}
}
