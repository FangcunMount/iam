package identity

import identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"

// ProfileClient 档案命令服务客户端。
type ProfileClient struct {
	commandService identityv2.ProfileCommandClient
}

// NewProfileClient 创建档案命令客户端。
func NewProfileClient(command identityv2.ProfileCommandClient) *ProfileClient {
	return &ProfileClient{commandService: command}
}
