package identity

import (
	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	"google.golang.org/grpc"
)

// NewClientFromConn 使用已有 gRPC 连接创建身份服务客户端。
func NewClientFromConn(conn grpc.ClientConnInterface) *Client {
	return NewClient(
		identityv2.NewIdentityReadClient(conn),
		identityv2.NewIdentityLifecycleClient(conn),
	)
}

// NewProfileClientFromConn 使用已有 gRPC 连接创建档案命令客户端。
func NewProfileClientFromConn(conn grpc.ClientConnInterface) *ProfileClient {
	return NewProfileClient(identityv2.NewProfileCommandClient(conn))
}

// NewProfileLinkClientFromConn 使用已有 gRPC 连接创建档案关系客户端。
func NewProfileLinkClientFromConn(conn grpc.ClientConnInterface) *ProfileLinkClient {
	return NewProfileLinkClient(
		identityv2.NewProfileLinkQueryClient(conn),
		identityv2.NewProfileLinkCommandClient(conn),
	)
}
