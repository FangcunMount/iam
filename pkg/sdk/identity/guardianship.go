package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// GuardianshipClient 监护关系服务客户端。
type GuardianshipClient struct {
	queryService   identityv1.GuardianshipQueryClient
	commandService identityv1.GuardianshipCommandClient
}

// NewGuardianshipClient 创建监护关系客户端。
func NewGuardianshipClient(query identityv1.GuardianshipQueryClient, command identityv1.GuardianshipCommandClient) *GuardianshipClient {
	return &GuardianshipClient{
		queryService:   query,
		commandService: command,
	}
}
