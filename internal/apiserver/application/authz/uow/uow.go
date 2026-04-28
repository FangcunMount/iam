package uow

import (
	"context"

	"gorm.io/gorm"

	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	assignmentrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/assignment"
	casbinrulerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/casbinrule"
	policyrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/role"
	userrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	txpkg "github.com/FangcunMount/iam/internal/pkg/database/tx"
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

var _ txpkg.UnitOfWork[TxRepositories] = (*gormUnitOfWork)(nil)

func NewUnitOfWork(db *gorm.DB) UnitOfWork {
	return &gormUnitOfWork{base: dbmysql.NewUnitOfWork(db)}
}

type gormUnitOfWork struct {
	base *dbmysql.UnitOfWork
}

func (u *gormUnitOfWork) WithinTx(ctx context.Context, fn func(tx TxRepositories) error) error {
	if u == nil || u.base == nil {
		return fn(TxRepositories{})
	}
	return u.base.WithinTransaction(ctx, func(tx *gorm.DB) error {
		repos := TxRepositories{
			Assignments:    assignmentrepo.NewAssignmentRepository(tx),
			Roles:          rolerepo.NewRoleRepository(tx),
			Resources:      resourcerepo.NewResourceRepository(tx),
			PolicyVersions: policyrepo.NewPolicyVersionRepository(tx),
			Users:          userrepo.NewRepository(tx),
			RuleStore:      casbinrulerepo.NewRepository(tx),
		}
		return fn(repos)
	})
}
