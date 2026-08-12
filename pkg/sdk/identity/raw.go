package identity

import identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"

// ReadRaw 返回原始读取服务客户端。
func (c *Client) ReadRaw() identityv2.IdentityReadClient {
	return c.readService
}

// LifecycleRaw 返回原始生命周期服务客户端。
func (c *Client) LifecycleRaw() identityv2.IdentityLifecycleClient {
	return c.lifecycleService
}
