package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// QueryRaw 返回原始监护关系查询客户端。
func (c *GuardianshipClient) QueryRaw() identityv1.GuardianshipQueryClient {
	return c.queryService
}

// CommandRaw 返回原始监护关系命令客户端。
func (c *GuardianshipClient) CommandRaw() identityv1.GuardianshipCommandClient {
	return c.commandService
}
