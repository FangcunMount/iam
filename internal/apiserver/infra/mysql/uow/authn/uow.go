package authn

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	acctrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/account"
	credentialrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/credential"
	profilerepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/profile"
	profileLinkRepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/profilelink"
	userrepo "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
)

var _ appuow.UnitOfWork = (*unitOfWork)(nil)

// NewUnitOfWork 创建基于 MySQL/GORM 的认证事务边界。
func NewUnitOfWork(db *gorm.DB) appuow.UnitOfWork {
	return &unitOfWork{
		base: mysql.NewUnitOfWork(db),
	}
}

type unitOfWork struct {
	base *mysql.UnitOfWork
}

func (u *unitOfWork) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx appuow.TxRepositories) error) error {
	if fn == nil {
		return nil
	}
	if u == nil || u.base == nil {
		return mysql.ErrUnitOfWorkUnavailable
	}

	return u.base.WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := mysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		repos := appuow.TxRepositories{
			Accounts:     acctrepo.NewAccountRepository(tx),
			Credentials:  credentialrepo.NewRepository(tx),
			Profiles:     profilerepo.NewRepository(tx),
			ProfileLinks: profileLinkRepo.NewRepository(tx),
			Users:        userrepo.NewRepository(tx),
		}
		return fn(txCtx, repos)
	})
}
