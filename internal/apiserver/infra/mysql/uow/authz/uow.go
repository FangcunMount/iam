package authz

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	assignmentrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/assignment"
	casbinrulerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/casbinrule"
	policyrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/role"
	userrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
)

var _ appuow.UnitOfWork = (*unitOfWork)(nil)

// NewUnitOfWork 创建基于 MySQL/GORM 的授权事务边界。
func NewUnitOfWork(db *gorm.DB) appuow.UnitOfWork {
	return &unitOfWork{base: dbmysql.NewUnitOfWork(db)}
}

type unitOfWork struct {
	base *dbmysql.UnitOfWork
}

func (u *unitOfWork) WithinTx(ctx context.Context, fn func(tx appuow.TxRepositories) error) error {
	if u == nil || u.base == nil {
		return fn(appuow.TxRepositories{})
	}
	return u.base.WithinTransaction(ctx, func(tx *gorm.DB) error {
		repos := appuow.TxRepositories{
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
