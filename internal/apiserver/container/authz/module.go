package authz

import (
	"fmt"
	"strings"

	assignmentAuthApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentauth"
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
	authzshared "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/shared"
	assignmentConstraints "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/assignmentconstraints"
	"github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
)

// AuthzModule 授权模块
type AuthzModule struct {
	routeAuthorization          authn.RouteAuthorizationRuntime
	roleNames                   RoleNameReader
	runtimeHealth               RuntimeHealthReporter
	policyReloader              authzshared.RuntimePolicyReloader
	resourceCatalog             resourceApp.Catalog
	resourceDirectory           resourceApp.Directory
	roleCatalog                 roleApp.Catalog
	roleDirectory               roleApp.Directory
	permissionGrantService      *permissionGrantApp.Service
	roleInheritanceService      *roleInheritanceApp.Service
	roleBindingCommands         bindingApp.Commands
	roleBindingDirectory        bindingApp.Directory
	authorizationChecker        *authorizationApp.NativeChecker
	authorizationSnapshotReader *authorizationApp.NativeSnapshotReader
	assignmentRequestAuthorizer assignmentAuthApp.Authorizer
}

// NewAuthzModule 创建授权模块
func NewAuthzModule() *AuthzModule {
	return &AuthzModule{}
}

// InitializeWithDeps 初始化授权模块。
func (m *AuthzModule) InitializeWithDeps(deps AuthzModuleDeps) error {
	if deps.DB == nil {
		return fmt.Errorf("mysql db is required")
	}
	if deps.EventStager == nil {
		return fmt.Errorf("authz event stager is required")
	}
	if deps.UserResolver == nil {
		return fmt.Errorf("identity user resolver is required")
	}

	infra, err := m.initializeInfrastructure(deps.DB, deps.EventStager, deps.UserResolver)
	if err != nil {
		return err
	}
	domain := m.initializeDomain(infra, deps.UserResolver)
	m.initializeRuntime(infra)
	m.initializeApplication(infra, domain)
	if strings.TrimSpace(deps.AssignmentConstraintsFile) != "" {
		if deps.GRPCACLEnabled {
			m.assignmentRequestAuthorizer, err = assignmentConstraints.LoadWithACL(
				deps.AssignmentConstraintsFile,
				deps.GRPCACLConfigFile,
			)
		} else {
			m.assignmentRequestAuthorizer, err = assignmentConstraints.Load(deps.AssignmentConstraintsFile)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *AuthzModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		ResourceCatalog:             m.resourceCatalog,
		ResourceDirectory:           m.resourceDirectory,
		RoleCatalog:                 m.roleCatalog,
		RoleDirectory:               m.roleDirectory,
		PermissionGrantService:      m.permissionGrantService,
		RoleInheritanceService:      m.roleInheritanceService,
		RoleBindingCommands:         m.roleBindingCommands,
		RoleBindingDirectory:        m.roleBindingDirectory,
		RouteAuthorization:          m.routeAuthorization,
		RuntimeHealth:               m.runtimeHealth,
		AuthorizationChecker:        m.authorizationChecker,
		AuthorizationSnapshotReader: m.authorizationSnapshotReader,
		AssignmentRequestAuthorizer: m.assignmentRequestAuthorizer,
	}
}

func (m *AuthzModule) RoleNameReader() RoleNameReader {
	if m == nil {
		return nil
	}
	return m.roleNames
}
