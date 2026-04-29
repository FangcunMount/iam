package uow

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

// TxRepositories 聚合事务中可使用的仓储集合。
type TxRepositories struct {
	Guardianships guardianship.Repository
	Children      child.Repository
	Users         user.Repository
}

// UnitOfWork 提供业务事务边界。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
