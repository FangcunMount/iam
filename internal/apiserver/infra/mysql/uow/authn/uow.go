package authn

import (
	"context"

	"gorm.io/gorm"

	appuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/uow"
	credentialrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/credential"
	loginidentityrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/loginidentity"
	profilerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/profile"
	profileLinkRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/profilelink"
	userrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
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
			Credentials:     credentialrepo.NewRepository(tx),
			LoginIdentities: loginidentityrepo.NewRepository(tx),
			Profiles:        profilerepo.NewRepository(tx),
			ProfileLinks:    profileLinkRepo.NewRepository(tx),
			Users:           userrepo.NewRepository(tx),
		}
		return fn(txCtx, repos)
	})
}
