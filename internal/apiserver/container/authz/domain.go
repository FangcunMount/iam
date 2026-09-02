package authz

import (
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	authorizationDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
)

type authzDomainComponents struct {
	authorizationEvaluator authorizationDomain.Evaluator
	assignmentValidator    assignmentDomain.Validator
}

func (m *AuthzModule) initializeDomain(infra *authzInfrastructureComponents, userResolver useraccess.UserResolver) *authzDomainComponents {
	subjectResolver := assignmentDomain.NewSubjectResolverRegistry(
		assignmentDomain.NewUserSubjectResolver(userResolver),
	)
	return &authzDomainComponents{
		authorizationEvaluator: authorizationDomain.NewEvaluator(),
		assignmentValidator:    assignmentDomain.NewValidatorWithSubjectResolver(infra.roleRepository, subjectResolver),
	}
}
