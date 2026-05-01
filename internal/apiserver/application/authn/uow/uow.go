package uow

import (
	"context"

	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	credentialDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	profileDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profilelink"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
)

// TxRepositories 聚合事务中可使用的仓储集合。
type TxRepositories struct {
	Accounts     accountDomain.Repository
	Credentials  credentialDomain.Repository
	Profiles     profileDomain.Repository
	ProfileLinks profileLinkDomain.Repository
	Users        userDomain.Repository
}

// UnitOfWork 提供业务事务边界。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
