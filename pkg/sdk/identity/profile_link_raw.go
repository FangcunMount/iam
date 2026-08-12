package identity

import identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"

// QueryRaw 返回原始档案关系查询客户端。
func (c *ProfileLinkClient) QueryRaw() identityv2.ProfileLinkQueryClient {
	return c.queryService
}

// CommandRaw 返回原始档案关系命令客户端。
func (c *ProfileLinkClient) CommandRaw() identityv2.ProfileLinkCommandClient {
	return c.commandService
}
