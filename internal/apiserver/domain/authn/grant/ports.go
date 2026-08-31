package grant

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
)

// Issuer 将已认证主体颁发为完整在线认证结果（Session + TokenSet）。
type Issuer interface {
	Issue(ctx context.Context, principal *authentication.Principal) (*AuthenticationGrant, error)
}

// SessionCreator 是建立认证结果所需的最小 Session 创建能力。
type SessionCreator interface {
	Create(ctx context.Context, principal *authentication.Principal) (*sessiondomain.Session, error)
}

// TokenSetMinter 是建立认证结果所需的用户令牌签发能力。
type TokenSetMinter interface {
	MintTokenSet(ctx context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*tokendomain.UserTokenSet, error)
}

// RefreshTokenSaver 是建立认证结果所需的最小令牌持久化能力。
type RefreshTokenSaver interface {
	SaveRefreshToken(ctx context.Context, token *tokendomain.RefreshToken) error
}
