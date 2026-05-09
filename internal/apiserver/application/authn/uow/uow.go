package uow

import (
	"context"

	credentialDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	loginidentityDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	profileDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
)

// TxRepositories 聚合事务中可使用的仓储集合。
type TxRepositories struct {
	Credentials     credentialDomain.Repository
	LoginIdentities loginidentityDomain.Repository
	Profiles        profileDomain.Repository
	ProfileLinks    profileLinkDomain.Repository
	Users           userDomain.Repository
}

// UnitOfWork 提供业务事务边界。
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(txCtx context.Context, tx TxRepositories) error) error
}
