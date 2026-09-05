package authz

import (
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	authorizationDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
)

type authzDomainComponents struct {
	authorizationEvaluator authorizationDomain.Evaluator
	assignmentValidator    assignmentDomain.Validator
}

func (m *AuthzModule) initializeDomain(infra *authzInfrastructureComponents) *authzDomainComponents {
	return &authzDomainComponents{
		authorizationEvaluator: authorizationDomain.NewEvaluator(),
		assignmentValidator:    assignmentDomain.NewValidatorWithSubjectResolver(infra.roleRepository, infra.subjectResolver),
	}
}
