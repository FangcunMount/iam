package identity

import (
	"google.golang.org/grpc"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	childApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	userApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	childDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	guardianshipDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

// Service 聚合 identity 模块的 gRPC 服务
type Service struct {
	identityRead      identityReadServer
	guardianshipQry   guardianshipQueryServer
	guardianshipCmd   guardianshipCommandServer
	identityLifecycle identityLifecycleServer
}

// NewService 创建 identity gRPC 服务
// 参数：
//   - userRepo: 用户领域仓储
//   - childRepo: 儿童领域仓储
//   - guardRepo: 监护关系领域仓储
//   - userQuerySvc: 用户查询应用服务
//   - childQuerySvc: 儿童查询应用服务
//   - guardianshipQuerySvc: 监护关系查询应用服务
//   - userSvc: 用户应用服务
//   - userProfileSvc: 用户资料应用服务
//   - userStatusSvc: 用户状态应用服务
//   - guardianshipSvc: 监护关系应用服务
func NewService(
	userRepo userDomain.Repository,
	childRepo childDomain.Repository,
	guardRepo guardianshipDomain.Repository,
	userQuerySvc userApp.UserQueryApplicationService,
	childQuerySvc childApp.ChildQueryApplicationService,
	guardianshipQuerySvc guardianshipApp.GuardianshipQueryApplicationService,
	userSvc userApp.UserApplicationService,
	userProfileSvc userApp.UserProfileApplicationService,
	userStatusSvc userApp.UserStatusApplicationService,
	guardianshipSvc guardianshipApp.GuardianshipApplicationService,
) *Service {
	return &Service{
		identityRead: identityReadServer{
			userRepo:      userRepo,
			childRepo:     childRepo,
			userQuerySvc:  userQuerySvc,
			childQuerySvc: childQuerySvc,
		},
		guardianshipQry: guardianshipQueryServer{
			childRepo:            childRepo,
			guardRepo:            guardRepo,
			guardianshipQuerySvc: guardianshipQuerySvc,
			userQuerySvc:         userQuerySvc,
		},
		guardianshipCmd: guardianshipCommandServer{
			guardianshipSvc:      guardianshipSvc,
			guardianshipQuerySvc: guardianshipQuerySvc,
			guardRepo:            guardRepo,
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
	identityv1.RegisterGuardianshipQueryServer(server, &s.guardianshipQry)
	identityv1.RegisterGuardianshipCommandServer(server, &s.guardianshipCmd)
	identityv1.RegisterIdentityLifecycleServer(server, &s.identityLifecycle)
}

// ============= 服务器结构体定义 =============

// identityReadServer 用户和儿童身份读取服务
type identityReadServer struct {
	identityv1.UnimplementedIdentityReadServer
	userRepo      userDomain.Repository
	childRepo     childDomain.Repository
	userQuerySvc  userApp.UserQueryApplicationService
	childQuerySvc childApp.ChildQueryApplicationService
}

// guardianshipQueryServer 监护关系查询服务
type guardianshipQueryServer struct {
	identityv1.UnimplementedGuardianshipQueryServer
	childRepo            childDomain.Repository
	guardRepo            guardianshipDomain.Repository
	guardianshipQuerySvc guardianshipApp.GuardianshipQueryApplicationService
	userQuerySvc         userApp.UserQueryApplicationService
}

// guardianshipCommandServer 监护关系命令服务（写操作）
type guardianshipCommandServer struct {
	identityv1.UnimplementedGuardianshipCommandServer
	guardianshipSvc      guardianshipApp.GuardianshipApplicationService
	guardianshipQuerySvc guardianshipApp.GuardianshipQueryApplicationService
	guardRepo            guardianshipDomain.Repository
}

// identityLifecycleServer 身份生命周期服务（用户管理）
type identityLifecycleServer struct {
	identityv1.UnimplementedIdentityLifecycleServer
	userSvc        userApp.UserApplicationService
	userQuerySvc   userApp.UserQueryApplicationService
	userProfileSvc userApp.UserProfileApplicationService
	userStatusSvc  userApp.UserStatusApplicationService
}
