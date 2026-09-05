package uow

import (
	"context"

	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	permissionGrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	roleInheritanceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

type TxRepositories struct {
	Assignments      assignmentDomain.Repository
	Roles            roleDomain.Repository
	Resources        resourceDomain.Repository
	PolicyVersions   policyDomain.Repository
	SubjectResolver  assignmentDomain.SubjectResolver
	PermissionGrants permissionGrantDomain.Repository
	RoleInheritances roleInheritanceDomain.Repository
	Events           event.Stager
}

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
