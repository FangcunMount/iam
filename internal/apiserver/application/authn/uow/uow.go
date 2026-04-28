package uow

import (
	"context"

	"gorm.io/gorm"

	accountDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	credentialDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	acctrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/account"
	credentialrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/credential"
	userrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/database/tx"
)

// TxRepositories 聚合事务中可使用的仓储集合。
type TxRepositories struct {
	Accounts    accountDomain.Repository
	Credentials credentialDomain.Repository
	Users       userDomain.Repository
}

// UnitOfWork 提供业务事务边界。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(tx TxRepositories) error) error
}

var _ tx.UnitOfWork[TxRepositories] = (*gormUnitOfWork)(nil)

// NewUnitOfWork 创建基于 GORM 的 UnitOfWork。
func NewUnitOfWork(db *gorm.DB) UnitOfWork {
	return &gormUnitOfWork{
		base: mysql.NewUnitOfWork(db),
	}
}

type gormUnitOfWork struct {
	base *mysql.UnitOfWork
}

func (u *gormUnitOfWork) WithinTx(ctx context.Context, fn func(tx TxRepositories) error) error {
	if u == nil || u.base == nil {
		return fn(TxRepositories{})
	}

	return u.base.WithinTransaction(ctx, func(tx *gorm.DB) error {
		repos := TxRepositories{
			Accounts:    acctrepo.NewAccountRepository(tx),
			Credentials: credentialrepo.NewRepository(tx),
			Users:       userrepo.NewRepository(tx),
		}
		return fn(repos)
	})
}
