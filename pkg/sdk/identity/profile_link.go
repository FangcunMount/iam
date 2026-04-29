package identity

import identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"

// ProfileLinkClient 档案关系服务客户端。
type ProfileLinkClient struct {
	queryService   identityv1.ProfileLinkQueryClient
	commandService identityv1.ProfileLinkCommandClient
}

// NewProfileLinkClient 创建档案关系客户端。
func NewProfileLinkClient(query identityv1.ProfileLinkQueryClient, command identityv1.ProfileLinkCommandClient) *ProfileLinkClient {
	return &ProfileLinkClient{
		queryService:   query,
		commandService: command,
	}
}
