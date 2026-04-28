package uow

import (
	"context"

	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

type TxRepositories struct {
	Assignments    assignmentDomain.Repository
	Roles          roleDomain.Repository
	Resources      resourceDomain.Repository
	PolicyVersions policyDomain.Repository
	Users          userDomain.Repository
	RuleStore      policyDomain.RuleStore
}

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(tx TxRepositories) error) error
}
