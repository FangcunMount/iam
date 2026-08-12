package authz

import (
	"fmt"
	"strings"

	assignmentAuthApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentauth"
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policy"
	policylintApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policylint"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	assignmentConstraints "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/assignmentconstraints"
	"github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
)

// AuthzModule 授权模块
type AuthzModule struct {
	routeAuthorization          authn.RouteAuthorizationRuntime
	roleNames                   RoleNameReader
	runtimeHealth               RuntimeHealthReporter
	policyReloader              policyApp.RuntimePolicyReloader
	resourceCatalog             resourceApp.Catalog
	resourceDirectory           resourceApp.Directory
	roleCatalog                 roleApp.Catalog
	roleDirectory               roleApp.Directory
	permissionCommands          policyApp.PermissionCommands
	permissionReader            policyApp.PermissionReader
	policyLinter                *policylintApp.Linter
	roleBindingCommands         bindingApp.Commands
	roleBindingDirectory        bindingApp.Directory
	authorizationChecker        *authorizationApp.Checker
	authorizationSnapshotReader *authorizationApp.SnapshotReader
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

	infra, err := m.initializeInfrastructure(deps.DB, deps.EventStager, authzModelPath(deps.ModelPath))
	if err != nil {
		return err
	}
	domain := m.initializeDomain(infra)
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

func authzModelPath(modelPath string) string {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return "configs/casbin_model.conf"
	}
	return modelPath
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
		PermissionCommands:          m.permissionCommands,
		PermissionReader:            m.permissionReader,
		PolicyLinter:                m.policyLinter,
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
