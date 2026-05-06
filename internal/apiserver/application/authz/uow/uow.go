package uow

import (
	"context"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

type AuthorizationFactStore interface {
	AddPermission(ctx context.Context, permission authzDomain.Permission) error
	RemovePermission(ctx context.Context, permission authzDomain.Permission) error
	AddRoleBinding(ctx context.Context, binding authzDomain.RoleBinding) error
	RemoveRoleBinding(ctx context.Context, binding authzDomain.RoleBinding) error
}

type TxRepositories struct {
	Bindings           bindingDomain.Repository
	Roles              roleDomain.Repository
	Resources          resourceDomain.Repository
	PolicyVersions     policyDomain.Repository
	Users              userDomain.Repository
	AuthorizationFacts AuthorizationFactStore
	Events             event.Stager
}

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
