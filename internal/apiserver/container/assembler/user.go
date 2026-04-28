package assembler

import (
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	appchild "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	appguard "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	appregistration "github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	appuow "github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	appuser "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	childInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/child"
	guardianshipInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/guardianship"
	userInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	ucGrpc "github.com/FangcunMount/iam/internal/apiserver/interface/uc/grpc"
	identityGrpc "github.com/FangcunMount/iam/internal/apiserver/interface/uc/grpc/identity"
	"github.com/FangcunMount/iam/internal/apiserver/interface/uc/restful/handler"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

// UserModule 用户模块
// 负责组装用户相关的所有组件
type UserModule struct {
	// handler 层
	UserHandler         *handler.UserHandler
	ChildHandler        *handler.ChildHandler
	GuardianshipHandler *handler.GuardianshipHandler
	// gRPC 服务
	GRPCService *ucGrpc.Service
}

// NewUserModule 创建用户模块
func NewUserModule() *UserModule {
	return &UserModule{}
}

// Initialize 初始化模块
func (m *UserModule) Initialize(params ...interface{}) error {
	db := params[0].(*gorm.DB)
	if db == nil {
		return errors.WithCode(code.ErrModuleInitializationFailed, "database connection is nil")
	}
	var casbin authn.CasbinEnforcer
	var sessionManager sessiondomain.Manager
	if len(params) > 1 {
		for _, param := range params[1:] {
			switch v := param.(type) {
			case authn.CasbinEnforcer:
				casbin = v
			case sessiondomain.Manager:
				sessionManager = v
			}
		}
	}

	// 事务
	uow := appuow.NewUnitOfWork(db)

	// 初始化仓储层
	userRepo := userInfra.NewRepository(db)
	childRepo := childInfra.NewRepository(db)
	guardRepo := guardianshipInfra.NewRepository(db)

	// 用户应用服务（命令）
	userAppSrv := appuser.NewUserApplicationService(uow)
	userProfileAppSrv := appuser.NewUserProfileApplicationService(uow)
	userStatusSrv := appuser.NewUserStatusApplicationService(uow, sessionManager)

	// 用户查询服务
	userQuerySrv := appuser.NewUserQueryApplicationService(uow)

	// 儿童应用服务（命令）
	childAppSrv := appchild.NewChildApplicationService(uow)
	childProfileAppSrv := appchild.NewChildProfileApplicationService(uow)

	// 儿童查询服务
	childQuerySrv := appchild.NewChildQueryApplicationService(uow)

	// 监护关系应用服务
	guardAppSrv := appguard.NewGuardianshipApplicationService(uow)

	// 监护关系查询服务
	guardQuerySrv := appguard.NewGuardianshipQueryApplicationService(uow)

	// 组合注册服务（单事务创建 child + guardianship）
	registrationAppSrv := appregistration.NewChildRegistrationService(uow)

	// 初始化 handler 层
	m.UserHandler = handler.NewUserHandler(
		userAppSrv,
		userProfileAppSrv,
		userQuerySrv,
		casbin,
	)

	m.ChildHandler = handler.NewChildHandler(
		childAppSrv,
		childProfileAppSrv,
		registrationAppSrv,
		guardQuerySrv,
		childQuerySrv,
	)

	m.GuardianshipHandler = handler.NewGuardianshipHandler(
		guardAppSrv,
		guardQuerySrv,
	)

	// 初始化 gRPC 服务
	identitySvc := identityGrpc.NewService(
		userRepo,
		childRepo,
		guardRepo,
		userQuerySrv,
		childQuerySrv,
		guardQuerySrv,
		userAppSrv,
		userProfileAppSrv,
		userStatusSrv,
		guardAppSrv,
	)

	m.GRPCService = ucGrpc.NewService(identitySvc)

	return nil
}

// Cleanup 清理模块资源
func (m *UserModule) Cleanup() error {
	// 如果有需要清理的资源，在这里进行清理
	// 比如关闭数据库连接、释放缓存等
	return nil
}

// CheckHealth 检查模块健康状态
func (m *UserModule) CheckHealth() error {
	return nil
}

// ModuleInfo 返回模块信息
func (m *UserModule) ModuleInfo() ModuleInfo {
	return ModuleInfo{
		Name:        "user",
		Version:     "1.0.0",
		Description: "用户管理模块",
	}
}
