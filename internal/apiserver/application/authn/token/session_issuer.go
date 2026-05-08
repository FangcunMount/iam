package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// sessionTokenIssuer 签发用户会话令牌
type sessionTokenIssuer struct {
	sessionManager SessionManager
	pairIssuer     SessionTokenPairIssuer
	refreshTTL     time.Duration
}

// 确保 sessionTokenIssuer 实现 SessionTokenIssuer 接口
var _ SessionTokenIssuer = (*sessionTokenIssuer)(nil)

// NewSessionTokenIssuer 创建 sessionTokenIssuer
func newSessionTokenIssuer(sessionManager SessionManager, pairIssuer SessionTokenPairIssuer, refreshTTL time.Duration) *sessionTokenIssuer {
	return &sessionTokenIssuer{
		sessionManager: sessionManager,
		pairIssuer:     pairIssuer,
		refreshTTL:     refreshTTL,
	}
}

// IssueToken 签发用户会话令牌
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
