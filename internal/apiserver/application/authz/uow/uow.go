package uow

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

type AuthorizationFactStore interface {
	AddPermission(ctx context.Context, permission permission.Permission) error
	RemovePermission(ctx context.Context, permission permission.Permission) error
	AddRoleBinding(ctx context.Context, binding bindingDomain.Fact) error
	RemoveRoleBinding(ctx context.Context, binding bindingDomain.Fact) error
}

type TxRepositories struct {
	Bindings           bindingDomain.Repository
	Roles              roleDomain.Repository
	Resources          resourceDomain.Repository
	PolicyVersions     policyDomain.Repository
	UserResolver       useraccess.UserResolver
	AuthorizationFacts AuthorizationFactStore
	Events             event.Stager
}

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
