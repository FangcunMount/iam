package authz

import (
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
)

type authzDomainComponents struct {
	resourceValidator    resourceDomain.Validator
	roleValidator        roleDomain.Validator
	roleBindingValidator bindingDomain.Validator
}

func (m *AuthzModule) initializeDomain(infra *authzInfrastructureComponents, userResolver useraccess.UserResolver) *authzDomainComponents {
	subjectResolver := bindingDomain.NewSubjectResolverRegistry(
		bindingDomain.NewUserSubjectResolver(userResolver),
	)
	return &authzDomainComponents{
		resourceValidator:    resourceDomain.NewValidator(infra.resourceRepository),
		roleValidator:        roleDomain.NewValidator(infra.roleRepository),
		roleBindingValidator: bindingDomain.NewValidatorWithSubjectResolver(infra.bindingRepository, infra.roleRepository, subjectResolver),
	}
}
