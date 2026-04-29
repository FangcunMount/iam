package mysql

import (
	"context"

	shareduow "github.com/FangcunMount/iam/pkg/uow/gorm"
	"gorm.io/gorm"
)

var (
	ErrUnitOfWorkUnavailable     = shareduow.ErrUnitOfWorkUnavailable
	ErrActiveTransactionRequired = shareduow.ErrActiveTransactionRequired
)

type TxOptions = shareduow.TxOptions
type UnitOfWork = shareduow.UnitOfWork

func NewUnitOfWork(db *gorm.DB) *UnitOfWork {
	return shareduow.NewUnitOfWork(db)
}

func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	return shareduow.TxFromContext(ctx)
}

func RequireTx(ctx context.Context) (*gorm.DB, error) {
	return shareduow.RequireTx(ctx)
}

func AfterCommit(ctx context.Context, hook func(context.Context) error) error {
	return shareduow.AfterCommit(ctx, hook)
}
