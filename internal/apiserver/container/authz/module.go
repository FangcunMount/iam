package authz

import (
	"fmt"
	"strings"
	"sync"

	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	assignmentAdmissionApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentadmission"
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	objectattributeadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	policychange "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
	assignmentConstraints "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/assignmentconstraints"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/attributeproviders"
)

// AuthzModule 授权模块
type AuthzModule struct {
	syncOnce                       sync.Once
	policySync                     *policySyncSubscriber
	routeDecisionService           authorizationApp.RoutePermissionChecker
	effectiveRoles                 EffectiveRoleReader
	runtimeHealth                  RuntimeHealthReporter
	policyReloader                 policychange.RuntimePolicyReloader
	resourceCatalog                resourceApp.Catalog
	resourceDirectory              resourceApp.Directory
	roleCatalog                    roleApp.Catalog
	roleDirectory                  roleApp.Directory
	permissionGrantService         *permissionGrantApp.Service
	roleInheritanceService         *roleInheritanceApp.Service
	assignmentCommands             assignmentApp.Commands
	assignmentDirectory            assignmentApp.Directory
	authorizationDecisions         *authorizationApp.DecisionService
	authorizationSnapshotReader    *authorizationApp.SnapshotReader
	assignmentAdmissionPolicy      assignmentAdmissionApp.Policy
	objectAttributeAdmissionPolicy objectattributeadmission.Policy
	attributeProviders             *objectattributeadmission.Registry
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

	providers, err := attributeproviders.Load(deps.AttributeProvidersFile)
	if err != nil {
		return err
	}
	m.attributeProviders = providers
	m.objectAttributeAdmissionPolicy = providers
	infra := m.initializeInfrastructure(deps.DB, deps.EventStager, deps.UserResolver)
	domain := m.initializeDomain(infra)
	if err := m.initializeRuntime(infra, domain, deps.SyncConfig); err != nil {
		return err
	}
	m.initializeApplication(infra, domain)
	if strings.TrimSpace(deps.AssignmentConstraintsFile) == "" {
		return fmt.Errorf("assignment constraints file is required")
	}
	if deps.GRPCACLEnabled {
		m.assignmentAdmissionPolicy, err = assignmentConstraints.LoadWithACL(
			deps.AssignmentConstraintsFile,
			deps.GRPCACLConfigFile,
		)
	} else {
		m.assignmentAdmissionPolicy, err = assignmentConstraints.Load(deps.AssignmentConstraintsFile)
	}
	if err != nil {
		return err
	}
	return nil
}

func (m *AuthzModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		ResourceCatalog:                m.resourceCatalog,
		ResourceDirectory:              m.resourceDirectory,
		RoleCatalog:                    m.roleCatalog,
		RoleDirectory:                  m.roleDirectory,
		PermissionGrantService:         m.permissionGrantService,
		RoleInheritanceService:         m.roleInheritanceService,
		AssignmentCommands:             m.assignmentCommands,
		AssignmentDirectory:            m.assignmentDirectory,
		RoutePermissionChecker:         m.routeDecisionService,
		RuntimeHealth:                  m.runtimeHealth,
		AuthorizationDecisions:         m.authorizationDecisions,
		AuthorizationSnapshotReader:    m.authorizationSnapshotReader,
		AssignmentAdmissionPolicy:      m.assignmentAdmissionPolicy,
		ObjectAttributeAdmissionPolicy: m.objectAttributeAdmissionPolicy,
	}
}

func (m *AuthzModule) EffectiveRoleReader() EffectiveRoleReader {
	if m == nil {
		return nil
	}
	return m.effectiveRoles
}
