package identity

import (
	"google.golang.org/grpc"

	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	profileApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profile"
	profileLinkApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profilelink"
	userApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/user"
)

// Service 聚合 identity 模块的 gRPC 服务
type Service struct {
	identityRead      identityReadServer
	profileCmd        profileCommandServer
	profileLinkQry    profileLinkQueryServer
	profileLinkCmd    profileLinkCommandServer
	identityLifecycle identityLifecycleServer
}

// NewService 创建 identity gRPC 服务
// 参数：
//   - userQuerySvc: 用户查询应用服务
//   - profileQuerySvc: 档案查询应用服务
//   - profileLinkQuerySvc: 档案关系查询应用服务
//   - userSvc: 用户应用服务
//   - userProfileSvc: 用户资料应用服务
//   - userStatusSvc: 用户状态应用服务
//   - profileCommandSvc: 当前用户视角档案创建用例
//   - profileLinkSvc: 档案关系应用服务
//   - profileLinkAccessSvc: 当前用户视角档案关系访问用例
func NewService(
	userQuerySvc userApp.Directory,
	profileQuerySvc profileApp.Directory,
	profileLinkQuerySvc profileLinkApp.Directory,
	userSvc userApp.Creator,
	userProfileSvc userApp.Editor,
	userStatusSvc userApp.StatusChanger,
	profileCommandSvc profileApp.MyProfiles,
	profileLinkSvc profileLinkApp.Commands,
	profileLinkAccessSvc profileLinkApp.MyProfileLinks,
) *Service {
	return &Service{
		identityRead: identityReadServer{
			userQuerySvc:    userQuerySvc,
			profileQuerySvc: profileQuerySvc,
		},
		profileLinkQry: profileLinkQueryServer{
			profileLinkQuerySvc: profileLinkQuerySvc,
			userQuerySvc:        userQuerySvc,
		},
		profileCmd: profileCommandServer{
			profileCommandSvc: profileCommandSvc,
		},
		profileLinkCmd: profileLinkCommandServer{
			profileLinkSvc:       profileLinkSvc,
			profileLinkQuerySvc:  profileLinkQuerySvc,
			profileLinkAccessSvc: profileLinkAccessSvc,
		},
		identityLifecycle: identityLifecycleServer{
			userSvc:        userSvc,
			userQuerySvc:   userQuerySvc,
			userProfileSvc: userProfileSvc,
			userStatusSvc:  userStatusSvc,
		},
	}
}

// Register 注册 gRPC 服务到 gRPC 服务器
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	identityv2.RegisterIdentityReadServer(server, &s.identityRead)
	identityv2.RegisterProfileLinkQueryServer(server, &s.profileLinkQry)
	identityv2.RegisterProfileCommandServer(server, &s.profileCmd)
	identityv2.RegisterProfileLinkCommandServer(server, &s.profileLinkCmd)
	identityv2.RegisterIdentityLifecycleServer(server, &s.identityLifecycle)
}

// ============= 服务器结构体定义 =============

// identityReadServer 用户和档案身份读取服务
type identityReadServer struct {
	identityv2.UnimplementedIdentityReadServer
	userQuerySvc    userApp.Directory
	profileQuerySvc profileApp.Directory
}

// profileLinkQueryServer 档案关系查询服务
type profileLinkQueryServer struct {
	identityv2.UnimplementedProfileLinkQueryServer
	profileLinkQuerySvc profileLinkApp.Directory
	userQuerySvc        userApp.Directory
}

// profileCommandServer 档案命令服务（写操作）
type profileCommandServer struct {
	identityv2.UnimplementedProfileCommandServer
	profileCommandSvc profileApp.MyProfiles
}

// profileLinkCommandServer 档案关系命令服务（写操作）
type profileLinkCommandServer struct {
	identityv2.UnimplementedProfileLinkCommandServer
	profileLinkSvc       profileLinkApp.Commands
	profileLinkQuerySvc  profileLinkApp.Directory
	profileLinkAccessSvc profileLinkApp.MyProfileLinks
}

// identityLifecycleServer 身份生命周期服务（用户管理）
type identityLifecycleServer struct {
	identityv2.UnimplementedIdentityLifecycleServer
	userSvc        userApp.Creator
	userQuerySvc   userApp.Directory
	userProfileSvc userApp.Editor
	userStatusSvc  userApp.StatusChanger
}
