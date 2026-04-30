package policy

import (
	"context"

	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

type PolicyCommandService struct {
	admin *PolicyAdministration
}

func NewPolicyCommandService(
	policyValidator policyDomain.Validator,
	uow authzuow.UnitOfWork,
	runtimeReloader RuntimePolicyReloader,
) *PolicyCommandService {
	return &PolicyCommandService{
		admin: NewPolicyAdministration(PolicyAdministrationDeps{
			PolicyValidator: policyValidator,
			UnitOfWork:      uow,
			RuntimeReloader: runtimeReloader,
		}),
	}
}

func (s *PolicyCommandService) AddPermission(
	ctx context.Context,
	cmd AddPermissionCommand,
) error {
	return s.admin.GrantPermissionToRole(ctx, cmd)
}

func (s *PolicyCommandService) RemovePermission(
	ctx context.Context,
	cmd RemovePermissionCommand,
) error {
	return s.admin.RevokePermissionFromRole(ctx, cmd)
}

var _ PermissionCommands = (*PolicyCommandService)(nil)
