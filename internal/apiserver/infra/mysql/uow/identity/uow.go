package identity

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profilerepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/profile"
	guardrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/profilelink"
	sessionrevocationrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/sessionrevocation"
	userrepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	dbmysql "github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
)

var _ appuow.UnitOfWork = (*unitOfWork)(nil)

// NewUnitOfWork 创建基于 MySQL/GORM 的身份域事务边界。
func NewUnitOfWork(db *gorm.DB) appuow.UnitOfWork {
	return &unitOfWork{
		base: dbmysql.NewUnitOfWork(db),
	}
}

type unitOfWork struct {
	base *dbmysql.UnitOfWork
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
			ProfileLinks:       guardrepo.NewRepository(tx),
			Profiles:           profilerepo.NewRepository(tx),
			Users:              userrepo.NewRepository(tx),
			SessionRevocations: sessionrevocationrepo.NewStore(tx),
		}
		return fn(txCtx, repos)
	})
}
