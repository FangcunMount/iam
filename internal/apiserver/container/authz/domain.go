package authz

import (
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	authorizationDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
)

type authzDomainComponents struct {
	authorizationEvaluator authorizationDomain.Evaluator
	resourceValidator      resourceDomain.Validator
	roleValidator          roleDomain.Validator
	assignmentValidator    assignmentDomain.Validator
}

func (m *AuthzModule) initializeDomain(infra *authzInfrastructureComponents, userResolver useraccess.UserResolver) *authzDomainComponents {
	subjectResolver := assignmentDomain.NewSubjectResolverRegistry(
		assignmentDomain.NewUserSubjectResolver(userResolver),
	)
	return &authzDomainComponents{
		authorizationEvaluator: authorizationDomain.NewEvaluator(),
		resourceValidator:      resourceDomain.NewValidator(infra.resourceRepository),
		roleValidator:          roleDomain.NewValidator(infra.roleRepository),
		assignmentValidator:    assignmentDomain.NewValidatorWithSubjectResolver(infra.assignmentRepository, infra.roleRepository, subjectResolver),
	}
}
