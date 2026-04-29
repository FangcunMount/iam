package assembler

import (
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	appchild "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	appguard "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	appregistration "github.com/FangcunMount/iam/internal/apiserver/application/uc/registration"
	appuser "github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	mysqlUcUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/uc"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

// UserModule 用户模块
// 负责组装用户相关的所有组件
type UserModule struct {
	userAppSrv         appuser.UserApplicationService
	userProfileAppSrv  appuser.UserProfileApplicationService
	userStatusSrv      appuser.UserStatusApplicationService
	userQuerySrv       appuser.UserQueryApplicationService
	childQuerySrv      appchild.ChildQueryApplicationService
	childAccessSrv     appchild.ChildAccessApplicationService
	guardAppSrv        appguard.GuardianshipApplicationService
	guardQuerySrv      appguard.GuardianshipQueryApplicationService
	guardAccessSrv     appguard.GuardianshipAccessApplicationService
	registrationAppSrv appregistration.ChildRegistrationService
	casbin             authn.CasbinEnforcer
}

// NewUserModule 创建用户模块
func NewUserModule() *UserModule {
	return &UserModule{}
}

type UserModuleDeps struct {
	DB             *gorm.DB
	Casbin         authn.CasbinEnforcer
	SessionManager sessiondomain.Manager
}

// InitializeWithDeps 初始化用户模块。
func (m *UserModule) InitializeWithDeps(deps UserModuleDeps) error {
	if deps.DB == nil {
		return errors.WithCode(code.ErrModuleInitializationFailed, "database connection is nil")
	}

	// 事务
	uow := mysqlUcUow.NewUnitOfWork(deps.DB)

	// 用户应用服务（命令）
	userAppSrv := appuser.NewUserApplicationService(uow)
	userProfileAppSrv := appuser.NewUserProfileApplicationService(uow)
	userStatusSrv := appuser.NewUserStatusApplicationService(uow, deps.SessionManager)

	// 用户查询服务
	userQuerySrv := appuser.NewUserQueryApplicationService(uow)

	// 儿童查询服务
	childQuerySrv := appchild.NewChildQueryApplicationService(uow)
	childAccessSrv := appchild.NewChildAccessApplicationService(uow)

	// 监护关系应用服务
	guardAppSrv := appguard.NewGuardianshipApplicationService(uow)

	// 监护关系查询服务
	guardQuerySrv := appguard.NewGuardianshipQueryApplicationService(uow)
	guardAccessSrv := appguard.NewGuardianshipAccessApplicationService(uow)

	// 组合注册服务（单事务创建 child + guardianship）
	registrationAppSrv := appregistration.NewChildRegistrationService(uow)

	m.userAppSrv = userAppSrv
	m.userProfileAppSrv = userProfileAppSrv
	m.userStatusSrv = userStatusSrv
	m.userQuerySrv = userQuerySrv
	m.childQuerySrv = childQuerySrv
	m.childAccessSrv = childAccessSrv
	m.guardAppSrv = guardAppSrv
	m.guardQuerySrv = guardQuerySrv
	m.guardAccessSrv = guardAccessSrv
	m.registrationAppSrv = registrationAppSrv
	m.casbin = deps.Casbin
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

func (m *UserModule) ApplicationCapabilities() UserApplicationCapabilities {
	if m == nil {
		return UserApplicationCapabilities{}
	}
	return UserApplicationCapabilities{
		UserService:               m.userAppSrv,
		UserProfileService:        m.userProfileAppSrv,
		UserStatusService:         m.userStatusSrv,
		UserQueryService:          m.userQuerySrv,
		ChildQueryService:         m.childQuerySrv,
		ChildAccessService:        m.childAccessSrv,
		GuardianshipService:       m.guardAppSrv,
		GuardianshipQueryService:  m.guardQuerySrv,
		GuardianshipAccessService: m.guardAccessSrv,
		ChildRegistrationService:  m.registrationAppSrv,
		Casbin:                    m.casbin,
	}
}
