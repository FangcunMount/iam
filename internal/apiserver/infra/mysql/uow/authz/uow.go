package authz

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	casbinrulerepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/casbinrule"
	policyrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/role"
	bindingrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/rolebinding"
	userrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	dbmysql "github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

var _ appuow.UnitOfWork = (*unitOfWork)(nil)

// NewUnitOfWork 创建基于 MySQL/GORM 的授权事务边界。
func NewUnitOfWork(db *gorm.DB, stagers ...event.Stager) appuow.UnitOfWork {
	var stager event.Stager
	if len(stagers) > 0 {
		stager = stagers[0]
	}
	return &unitOfWork{base: dbmysql.NewUnitOfWork(db), events: stager}
}

type unitOfWork struct {
	base   *dbmysql.UnitOfWork
	events event.Stager
}

func (u *unitOfWork) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx appuow.TxRepositories) error) error {
	if fn == nil {
		return nil
	}
	if u == nil || u.base == nil {
		return dbmysql.ErrUnitOfWorkUnavailable
	}
	return u.base.WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := dbmysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		repos := appuow.TxRepositories{
			Bindings:           bindingrepo.NewBindingRepository(tx),
			Roles:              rolerepo.NewRoleRepository(tx),
			Resources:          resourcerepo.NewResourceRepository(tx),
			PolicyVersions:     policyrepo.NewPolicyVersionRepository(tx),
			Users:              userrepo.NewRepository(tx),
			AuthorizationFacts: casbinrulerepo.NewRepository(tx),
			Events:             u.events,
		}
		return fn(txCtx, repos)
	})
}
