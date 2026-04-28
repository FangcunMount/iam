package uc

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	childrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/child"
	guardrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/guardianship"
	userrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
)

var _ appuow.UnitOfWork = (*unitOfWork)(nil)

// NewUnitOfWork 创建基于 MySQL/GORM 的用户中心事务边界。
func NewUnitOfWork(db *gorm.DB) appuow.UnitOfWork {
	return &unitOfWork{
		base: dbmysql.NewUnitOfWork(db),
	}
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
			Guardianships: guardrepo.NewRepository(tx),
			Children:      childrepo.NewRepository(tx),
			Users:         userrepo.NewRepository(tx),
		}
		return fn(repos)
	})
}
