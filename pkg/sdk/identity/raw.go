package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// ReadRaw 返回原始读取服务客户端。
func (c *Client) ReadRaw() identityv1.IdentityReadClient {
	return c.readService
}

// LifecycleRaw 返回原始生命周期服务客户端。
func (c *Client) LifecycleRaw() identityv1.IdentityLifecycleClient {
	return c.lifecycleService
}
