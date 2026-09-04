package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type revoker struct {
	tokenCodec     BearerTokenCodec
	tokenStore     Store
	sessionRevoker SessionRevoker
}

func newRevoker(tokenCodec BearerTokenCodec, tokenStore Store, sessionRevoker SessionRevoker) Revoker {
	return &revoker{tokenCodec: tokenCodec, tokenStore: tokenStore, sessionRevoker: sessionRevoker}
}

func (s *revoker) RevokeBearerToken(ctx context.Context, tokenValue string) error {
	claims, err := s.tokenCodec.VerifyBearerToken(ctx, tokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse token for revocation")
	}
	if claims.IsExpired() {
		return nil
	}
	expiry := time.Until(claims.ExpiresAt)
	if expiry <= 0 {
		return nil
	}
	if err := s.tokenStore.MarkBearerTokenRevoked(ctx, claims.TokenID, expiry); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to mark bearer token revoked")
	}
	if claims.TokenType == TokenTypeAccess && claims.SessionID != "" {
		if err := s.sessionRevoker.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}
	return nil
}
