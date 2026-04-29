package identity

import (
	"google.golang.org/grpc"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	profileApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	profileLinkApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/profilelink"
	userApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
)

// Service 聚合 identity 模块的 gRPC 服务
type Service struct {
	identityRead      identityReadServer
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
//   - profileLinkSvc: 档案关系应用服务
//   - profileLinkAccessSvc: 当前用户视角档案关系访问用例
func NewService(
	userQuerySvc userApp.Directory,
	profileQuerySvc profileApp.Directory,
	profileLinkQuerySvc profileLinkApp.Directory,
	userSvc userApp.Creator,
	userProfileSvc userApp.Editor,
	userStatusSvc userApp.StatusChanger,
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

// RegisterService 注册 gRPC 服务到 gRPC 服务器
func (s *Service) RegisterService(server *grpc.Server) {
	identityv1.RegisterIdentityReadServer(server, &s.identityRead)
	identityv1.RegisterProfileLinkQueryServer(server, &s.profileLinkQry)
	identityv1.RegisterProfileLinkCommandServer(server, &s.profileLinkCmd)
	identityv1.RegisterIdentityLifecycleServer(server, &s.identityLifecycle)
}

// ============= 服务器结构体定义 =============

// identityReadServer 用户和档案身份读取服务
type identityReadServer struct {
	identityv1.UnimplementedIdentityReadServer
	userQuerySvc    userApp.Directory
	profileQuerySvc profileApp.Directory
}

// profileLinkQueryServer 档案关系查询服务
type profileLinkQueryServer struct {
	identityv1.UnimplementedProfileLinkQueryServer
	profileLinkQuerySvc profileLinkApp.Directory
	userQuerySvc        userApp.Directory
}

// profileLinkCommandServer 档案关系命令服务（写操作）
type profileLinkCommandServer struct {
	identityv1.UnimplementedProfileLinkCommandServer
	profileLinkSvc       profileLinkApp.Commands
	profileLinkQuerySvc  profileLinkApp.Directory
	profileLinkAccessSvc profileLinkApp.MyProfileLinks
}

// identityLifecycleServer 身份生命周期服务（用户管理）
type identityLifecycleServer struct {
	identityv1.UnimplementedIdentityLifecycleServer
	userSvc        userApp.Creator
	userQuerySvc   userApp.Directory
	userProfileSvc userApp.Editor
	userStatusSvc  userApp.StatusChanger
}
