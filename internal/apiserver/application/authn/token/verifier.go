package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

type verifier struct {
	tokenCodec    AccessTokenCodec
	tokenStore    Store
	sessionManger SessionManager
	accessChecker SubjectAccessEvaluator
}

func NewVerifier(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
) Verifier {
	return &verifier{
		tokenCodec:    tokenCodec,
		tokenStore:    tokenStore,
		sessionManger: sessionManager,
		accessChecker: accessChecker,
	}
}

func (s *verifier) VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error) {
	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		log.Warnw("failed to parse access token", "error", err, "token_hint", sanitize.MaskToken(tokenValue))
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse access token")
	}
	if claims.IsExpired() {
		return nil, perrors.WithCode(code.ErrExpired, "access token has expired")
	}
	if claims.TokenType == TokenTypeService {
		return claims, nil
	}
	isRevoked, err := s.tokenStore.IsAccessTokenRevoked(ctx, claims.TokenID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to check revoked access token")
	}
	if isRevoked {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "access token has been revoked")
	}
	if claims.SessionID == "" {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "access token session is missing")
	}
	sess, err := s.sessionManger.Get(ctx, claims.SessionID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	if sess == nil || !sess.IsActive() {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "session has been revoked or expired")
	}
	decision, err := s.accessChecker.Evaluate(ctx, claims.UserID, claims.AccountID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}
	if !decision.IsAllowed() {
		return nil, subjectAccessVerifyError(decision.Status)
	}
	return claims, nil
}

func subjectAccessVerifyError(status sessiondomain.SubjectAccessStatus) error {
	switch status {
	case sessiondomain.SubjectAccessBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case sessiondomain.SubjectAccessDisabled:
		return perrors.WithCode(code.ErrCredentialDisabled, "account is disabled")
	case sessiondomain.SubjectAccessLocked:
		return perrors.WithCode(code.ErrCredentialLocked, "account is locked")
	default:
		return perrors.WithCode(code.ErrUserInactive, "user is inactive")
	}
}
