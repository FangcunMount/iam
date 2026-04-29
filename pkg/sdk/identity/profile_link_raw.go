package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// QueryRaw 返回原始档案关系查询客户端。
func (c *ProfileLinkClient) QueryRaw() identityv1.ProfileLinkQueryClient {
	return c.queryService
}

// CommandRaw 返回原始档案关系命令客户端。
func (c *ProfileLinkClient) CommandRaw() identityv1.ProfileLinkCommandClient {
	return c.commandService
}
