package assembler

import (
	"fmt"

	"gorm.io/gorm"

	assignmentApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/assignment"
	policyApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/role"
	authzUow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	casbinInfra "github.com/FangcunMount/iam/internal/apiserver/infra/casbin"
	assignmentInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/assignment"
	policyInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/role"
	userInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	authzgrpc "github.com/FangcunMount/iam/internal/apiserver/interface/authz/grpc"
	"github.com/FangcunMount/iam/internal/apiserver/interface/authz/restful/handler"
)

// AuthzModule 授权模块
type AuthzModule struct {
	// HTTP Handlers
	RoleHandler       *handler.RoleHandler
	AssignmentHandler *handler.AssignmentHandler
	PolicyHandler     *handler.PolicyHandler
	ResourceHandler   *handler.ResourceHandler
	CheckHandler      *handler.CheckHandler
	GRPCService       *authzgrpc.Service

	// CasbinAdapter 运行时策略引擎（供 HTTP/gRPC/中间件复用）
	CasbinAdapter policyDomain.CasbinAdapter
}

// NewAuthzModule 创建授权模块
func NewAuthzModule() *AuthzModule {
	return &AuthzModule{}
}

// Initialize 初始化授权模块
// versionNotifier: 策略版本通知器（可选，传 nil 则不发送通知）
func (m *AuthzModule) Initialize(db *gorm.DB, versionNotifier policyDomain.VersionNotifier) error {
	if db == nil {
		return fmt.Errorf("mysql db is required")
	}

	// 1. 初始化 Casbin Enforcer
	modelPath := "configs/casbin_model.conf"
	casbinAdapter, err := casbinInfra.NewCasbinAdapter(db, modelPath)
	if err != nil {
		return fmt.Errorf("failed to create casbin adapter: %w", err)
	}
	m.CasbinAdapter = casbinAdapter

	// 2. 初始化仓储层
	roleRepository := roleInfra.NewRoleRepository(db)
	assignmentRepository := assignmentInfra.NewAssignmentRepository(db)
	resourceRepository := resourceInfra.NewResourceRepository(db)
	policyVersionRepository := policyInfra.NewPolicyVersionRepository(db)
	userRepository := userInfra.NewRepository(db)
	unitOfWork := authzUow.NewUnitOfWork(db)

	// 3. 初始化领域服务
	// Resource 模块
	resourceManager := resourceDomain.NewValidator(resourceRepository)
	// Role 模块
	roleManager := roleDomain.NewValidator(roleRepository)
	// Policy 模块
	policyManager := policyDomain.NewValidator(roleRepository, resourceRepository)
	// Assignment 模块
	assignmentManager := assignmentDomain.NewValidator(assignmentRepository, roleRepository, userRepository)

	// 4. 初始化应用服务 - CQRS 分离
	// Resource 模块
	resourceCommander := resourceApp.NewResourceCommandService(resourceManager, resourceRepository)
	resourceQueryer := resourceApp.NewResourceQueryService(resourceRepository)
	// Role 模块
	roleCommander := roleApp.NewRoleCommandService(roleManager, roleRepository)
	roleQueryer := roleApp.NewRoleQueryService(roleRepository)
	// Policy 模块
	policyCommander := policyApp.NewPolicyCommandService(policyManager, unitOfWork, casbinAdapter, versionNotifier)
	policyQueryer := policyApp.NewPolicyQueryService(policyVersionRepository, casbinAdapter, roleRepository)
	// Assignment 模块
	assignmentCommander := assignmentApp.NewAssignmentCommandService(
		assignmentManager,
		unitOfWork,
		casbinAdapter,
		versionNotifier,
	)
	assignmentQueryer := assignmentApp.NewAssignmentQueryService(assignmentManager, assignmentRepository)

	// 5. 初始化 HTTP 处理器 - 依赖 driving 接口（CQRS）
	// Resource Handler
	m.ResourceHandler = handler.NewResourceHandler(resourceCommander, resourceQueryer)
	// Role Handler
	m.RoleHandler = handler.NewRoleHandler(roleCommander, roleQueryer)
	// Policy Handler
	m.PolicyHandler = handler.NewPolicyHandler(policyCommander, policyQueryer)
	// Assignment Handler
	m.AssignmentHandler = handler.NewAssignmentHandler(assignmentCommander, assignmentQueryer)
	// PDP
	m.CheckHandler = handler.NewCheckHandler(casbinAdapter)
	m.GRPCService = authzgrpc.NewService(casbinAdapter, roleRepository, policyVersionRepository, assignmentCommander)
	return nil
}
