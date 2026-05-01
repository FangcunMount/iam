package identity

import identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"

// ProfileLinkClient 档案关系服务客户端。
type ProfileLinkClient struct {
	queryService   identityv2.ProfileLinkQueryClient
	commandService identityv2.ProfileLinkCommandClient
}

// NewProfileLinkClient 创建档案关系客户端。
func NewProfileLinkClient(query identityv2.ProfileLinkQueryClient, command identityv2.ProfileLinkCommandClient) *ProfileLinkClient {
	return &ProfileLinkClient{
		queryService:   query,
		commandService: command,
	}
}
