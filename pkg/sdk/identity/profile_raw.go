package identity

import identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"

// CommandRaw 返回原始档案命令客户端。
func (c *ProfileClient) CommandRaw() identityv2.ProfileCommandClient {
	return c.commandService
}
