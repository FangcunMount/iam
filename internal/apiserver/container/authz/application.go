package authz

import (
	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	authorizationApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	permissionGrantApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/permissiongrant"
	policyVersionApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policyversion"
	resourceApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/role"
)

func (m *AuthzModule) initializeApplication(
	infra *authzInfrastructureComponents,
	domain *authzDomainComponents,
) {
	guard := management.NewGuard(infra.authorizationRuntime, m.assignmentAdmissionPolicy)
	m.authorizationDecisions = authorizationApp.NewDecisionService(infra.authorizationRuntime)
	m.resourceCatalog = resourceApp.NewResourceCatalog(infra.unitOfWork, infra.authorizationRuntime, m.authorizationDecisions)
	m.resourceDirectory = resourceApp.NewResourceQueryService(infra.resourceRepository)

	m.roleCatalog = roleApp.NewRoleCatalog(infra.unitOfWork, infra.authorizationRuntime, guard)
	m.roleDirectory = roleApp.NewRoleQueryService(infra.roleRepository, management.NewGuard(infra.authorizationRuntime))

	m.permissionGrantService = permissionGrantApp.NewService(
		infra.unitOfWork,
		infra.permissionGrantRepository,
		infra.authorizationRuntime, guard,
	)

	m.assignmentCommands = assignmentApp.NewCommandService(
		domain.assignmentValidator,
		infra.roleRepository,
		infra.unitOfWork,
		infra.authorizationRuntime, guard,
	)
	m.assignmentDirectory = assignmentApp.NewDirectory(domain.assignmentValidator, infra.assignmentRepository, infra.roleRepository, management.NewGuard(infra.authorizationRuntime))

	m.routeDecisionService = authorizationApp.NewRouteDecisionService(m.authorizationDecisions)
	m.authorizationSnapshotReader = authorizationApp.NewSnapshotReader(infra.authorizationRuntime)
	m.committedPolicyVersionReader = policyVersionApp.NewReader(infra.policyVersionRepository)
}
