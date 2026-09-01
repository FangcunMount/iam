package authz

import (
	authorizationDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
)

type authzDomainComponents struct {
	authorizationEvaluator authorizationDomain.Evaluator
	resourceValidator      resourceDomain.Validator
	roleValidator          roleDomain.Validator
	roleBindingValidator   bindingDomain.Validator
}

func (m *AuthzModule) initializeDomain(infra *authzInfrastructureComponents, userResolver useraccess.UserResolver) *authzDomainComponents {
	subjectResolver := bindingDomain.NewSubjectResolverRegistry(
		bindingDomain.NewUserSubjectResolver(userResolver),
	)
	return &authzDomainComponents{
		authorizationEvaluator: authorizationDomain.NewEvaluator(),
		resourceValidator:      resourceDomain.NewValidator(infra.resourceRepository),
		roleValidator:          roleDomain.NewValidator(infra.roleRepository),
		roleBindingValidator:   bindingDomain.NewValidatorWithSubjectResolver(infra.bindingRepository, infra.roleRepository, subjectResolver),
	}
}
