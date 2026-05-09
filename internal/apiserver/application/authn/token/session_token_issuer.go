package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ====================================================
// ================== Driving Ports ===================
// ====================================================
// sessionTokenIssuerPort 是登录成功后签发用户会话令牌的内部端口。
//
// 实现会创建 session、签发 access token，并保存 refresh token。
type sessionTokenIssuerPort interface {
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// ====================================================
// ================== Implementation ==================
// ====================================================

// sessionTokenIssuer 签发用户会话令牌
type sessionTokenIssuer struct {
	sessionManager SessionManager             // 会话管理器
	pairIssuer     sessionTokenPairIssuerPort // 令牌对签发器
	refreshTTL     time.Duration              // 刷新令牌有效期
}

// 确保 sessionTokenIssuer 实现 sessionTokenIssuerPort 接口。
var _ sessionTokenIssuerPort = (*sessionTokenIssuer)(nil)

// newSessionTokenIssuer 创建 sessionTokenIssuer。
func newSessionTokenIssuer(sessionManager SessionManager, pairIssuer sessionTokenPairIssuerPort, refreshTTL time.Duration) *sessionTokenIssuer {
	return &sessionTokenIssuer{
		sessionManager: sessionManager,
		pairIssuer:     pairIssuer,
		refreshTTL:     refreshTTL,
	}
}

// IssueToken 签发用户会话令牌。
func (s *sessionTokenIssuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	// 创建会话
	sessionExpiresAt := time.Now().Add(s.refreshTTL)
	sess, err := s.sessionManager.Create(ctx, principal, sessionExpiresAt)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	// 颁发令牌对
	return s.pairIssuer.IssueTokenPair(ctx, principal, sess)
}
