package uow

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/sessionrevocation"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
)

// TxRepositories 聚合事务中可使用的仓储集合。
type TxRepositories struct {
	ProfileLinks       profilelink.Repository
	Profiles           profile.Repository
	Users              user.Repository
	SessionRevocations sessionrevocation.Stager
}

// UnitOfWork 提供业务事务边界。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
