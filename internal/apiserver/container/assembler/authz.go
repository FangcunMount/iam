package assembler

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	authorizationApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

// AuthzModule 授权模块
type AuthzModule struct {
	routeAuthorization          authn.RouteAuthorizationRuntime
	roleNames                   RoleNameReader
	runtimeHealth               AuthzRuntimeHealthReporter
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
	if deps.EventStager == nil {
		return fmt.Errorf("authz event stager is required")
	}

	infra, err := m.initializeInfrastructure(deps.DB, deps.EventStager, authzModelPath(deps.ModelPath))
	if err != nil {
		return err
	}
	domain := m.initializeDomain(infra)
	m.initializeRuntime(infra)
	m.initializeApplication(infra, domain)
	return nil
}

func authzModelPath(modelPath string) string {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return "configs/casbin_model.conf"
	}
	return modelPath
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
		RouteAuthorization:          m.routeAuthorization,
		RuntimeHealth:               m.runtimeHealth,
		AuthorizationChecker:        m.authorizationChecker,
		AuthorizationSnapshotReader: m.authorizationSnapshotReader,
	}
}

func (m *AuthzModule) RoleNameReader() RoleNameReader {
	if m == nil {
		return nil
	}
	return m.roleNames
}
