package assembler

import (
	"fmt"

	"gorm.io/gorm"

	assignmentApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/assignment"
	policyApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/role"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	casbinInfra "github.com/FangcunMount/iam/internal/apiserver/infra/casbin"
	assignmentInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/assignment"
	policyInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/role"
	mysqlAuthzUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authz"
	userInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/pkg/event"
)

// AuthzModule 授权模块
type AuthzModule struct {
	// CasbinAdapter 运行时策略引擎（供 HTTP/gRPC/中间件复用）
	CasbinAdapter policyDomain.CasbinAdapter

	resourceCommander   resourceDomain.Commander
	resourceQueryer     resourceDomain.Queryer
	roleCommander       roleDomain.Commander
	roleQueryer         roleDomain.Queryer
	policyCommander     policyDomain.Commander
	policyQueryer       policyDomain.Queryer
	assignmentCommander assignmentDomain.Commander
	assignmentQueryer   assignmentDomain.Queryer
	roleRepository      roleDomain.Repository
	policyVersionRepo   policyDomain.Repository
}

// NewAuthzModule 创建授权模块
func NewAuthzModule() *AuthzModule {
	return &AuthzModule{}
}

type AuthzModuleDeps struct {
	DB          *gorm.DB
	EventStager event.Stager
	ModelPath   string
}

// InitializeWithDeps 初始化授权模块。
func (m *AuthzModule) InitializeWithDeps(deps AuthzModuleDeps) error {
	if deps.DB == nil {
		return fmt.Errorf("mysql db is required")
	}

	// 1. 初始化 Casbin Enforcer
	modelPath := deps.ModelPath
	if modelPath == "" {
		modelPath = "configs/casbin_model.conf"
	}
	casbinAdapter, err := casbinInfra.NewCasbinAdapter(deps.DB, modelPath)
	if err != nil {
		return fmt.Errorf("failed to create casbin adapter: %w", err)
	}
	m.CasbinAdapter = casbinAdapter

	// 2. 初始化仓储层
	roleRepository := roleInfra.NewRoleRepository(deps.DB)
	assignmentRepository := assignmentInfra.NewAssignmentRepository(deps.DB)
	resourceRepository := resourceInfra.NewResourceRepository(deps.DB)
	policyVersionRepository := policyInfra.NewPolicyVersionRepository(deps.DB)
	userRepository := userInfra.NewRepository(deps.DB)
	unitOfWork := mysqlAuthzUow.NewUnitOfWork(deps.DB, deps.EventStager)

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
	policyCommander := policyApp.NewPolicyCommandService(policyManager, unitOfWork, casbinAdapter)
	policyQueryer := policyApp.NewPolicyQueryService(policyVersionRepository, casbinAdapter, roleRepository)
	// Assignment 模块
	assignmentCommander := assignmentApp.NewAssignmentCommandService(
		assignmentManager,
		unitOfWork,
		casbinAdapter,
	)
	assignmentQueryer := assignmentApp.NewAssignmentQueryService(assignmentManager, assignmentRepository)

	m.resourceCommander = resourceCommander
	m.resourceQueryer = resourceQueryer
	m.roleCommander = roleCommander
	m.roleQueryer = roleQueryer
	m.policyCommander = policyCommander
	m.policyQueryer = policyQueryer
	m.assignmentCommander = assignmentCommander
	m.assignmentQueryer = assignmentQueryer
	m.roleRepository = roleRepository
	m.policyVersionRepo = policyVersionRepository
	return nil
}

func (m *AuthzModule) ApplicationCapabilities() AuthzApplicationCapabilities {
	if m == nil {
		return AuthzApplicationCapabilities{}
	}
	return AuthzApplicationCapabilities{
		ResourceCommander:   m.resourceCommander,
		ResourceQueryer:     m.resourceQueryer,
		RoleCommander:       m.roleCommander,
		RoleQueryer:         m.roleQueryer,
		PolicyCommander:     m.policyCommander,
		PolicyQueryer:       m.policyQueryer,
		AssignmentCommander: m.assignmentCommander,
		AssignmentQueryer:   m.assignmentQueryer,
		Casbin:              m.CasbinAdapter,
		RoleRepository:      m.roleRepository,
		PolicyVersionRepo:   m.policyVersionRepo,
	}
}
