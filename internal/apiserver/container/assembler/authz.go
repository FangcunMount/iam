package assembler

import (
	"fmt"

	"gorm.io/gorm"

	authorizationApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/internal/apiserver/application/authz/rolebinding"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/rolebinding"
	casbinInfra "github.com/FangcunMount/iam/internal/apiserver/infra/casbin"
	policyInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/role"
	bindingInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/rolebinding"
	mysqlAuthzUow "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/uow/authz"
	userInfra "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/pkg/event"
)

// AuthzModule 授权模块
type AuthzModule struct {
	// 授权运行时适配器（供中间件、健康检查和 application ports 复用）
	CasbinAdapter *casbinInfra.CasbinAdapter

	resourceCatalog             resourceApp.Catalog
	resourceDirectory           resourceApp.Directory
	roleCatalog                 roleApp.Catalog
	roleDirectory               roleApp.Directory
	permissionCommands          policyApp.PermissionCommands
	permissionReader            policyApp.PermissionReader
	roleBindingCommands         bindingApp.Commands
	roleBindingDirectory        bindingApp.Directory
	authorizationChecker        *authorizationApp.Checker
	authorizationSnapshotReader *authorizationApp.SnapshotReader
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

	// 1. 初始化 授权判定r
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
	bindingRepository := bindingInfra.NewBindingRepository(deps.DB)
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
	// Binding 模块
	bindingManager := bindingDomain.NewValidator(bindingRepository, roleRepository, userRepository)

	// 4. 初始化应用服务 - CQRS 分离
	// Resource 模块
	resourceCatalog := resourceApp.NewResourceCatalog(resourceManager, resourceRepository)
	resourceDirectory := resourceApp.NewResourceQueryService(resourceRepository)
	// Role 模块
	roleCatalog := roleApp.NewRoleCatalog(roleManager, roleRepository)
	roleDirectory := roleApp.NewRoleQueryService(roleRepository)
	// Policy 模块
	permissionCommands := policyApp.NewPolicyCommandService(policyManager, unitOfWork, casbinAdapter)
	permissionReader := policyApp.NewPolicyQueryService(policyVersionRepository, casbinAdapter, roleRepository)
	// Binding 模块
	roleBindingCommands := bindingApp.NewCommandService(
		bindingManager,
		roleRepository,
		unitOfWork,
		casbinAdapter,
	)
	roleBindingDirectory := bindingApp.NewDirectory(bindingManager, bindingRepository)
	authorizationChecker := authorizationApp.NewChecker(casbinAdapter)
	authorizationSnapshotReader := authorizationApp.NewSnapshotReader(casbinAdapter, policyVersionRepository)

	m.resourceCatalog = resourceCatalog
	m.resourceDirectory = resourceDirectory
	m.roleCatalog = roleCatalog
	m.roleDirectory = roleDirectory
	m.permissionCommands = permissionCommands
	m.permissionReader = permissionReader
	m.roleBindingCommands = roleBindingCommands
	m.roleBindingDirectory = roleBindingDirectory
	m.authorizationChecker = authorizationChecker
	m.authorizationSnapshotReader = authorizationSnapshotReader
	return nil
}

func (m *AuthzModule) ApplicationCapabilities() AuthzApplicationCapabilities {
	if m == nil {
		return AuthzApplicationCapabilities{}
	}
	return AuthzApplicationCapabilities{
		ResourceCatalog:             m.resourceCatalog,
		ResourceDirectory:           m.resourceDirectory,
		RoleCatalog:                 m.roleCatalog,
		RoleDirectory:               m.roleDirectory,
		PermissionCommands:          m.permissionCommands,
		PermissionReader:            m.permissionReader,
		RoleBindingCommands:         m.roleBindingCommands,
		RoleBindingDirectory:        m.roleBindingDirectory,
		RouteAuthorization:          m.CasbinAdapter,
		AuthorizationChecker:        m.authorizationChecker,
		AuthorizationSnapshotReader: m.authorizationSnapshotReader,
	}
}
