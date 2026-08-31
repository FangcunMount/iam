package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type refresher struct {
	tokenSetMinter     TokenSetMinter
	tokenStore         Store
	sessionLoader      SessionLoader
	sessionRevoker     SessionRevoker
	sessionExtender    SessionExtender
	admissionPolicy    AdmissionPolicy
	refreshClaimsCodec RefreshClaimsCodec
}

func newRefresher(tokenSetMinter TokenSetMinter, tokenStore Store, sessionLoader SessionLoader, sessionRevoker SessionRevoker, sessionExtender SessionExtender, admissionPolicy AdmissionPolicy, refreshClaimsCodec RefreshClaimsCodec) Refresher {
	return &refresher{
		tokenSetMinter: tokenSetMinter, tokenStore: tokenStore, sessionLoader: sessionLoader,
		sessionRevoker: sessionRevoker, sessionExtender: sessionExtender, admissionPolicy: admissionPolicy,
		refreshClaimsCodec: normalizeRefreshClaimsCodec(refreshClaimsCodec),
	}
}

func (s *refresher) RefreshToken(ctx context.Context, refreshTokenValue string) (*UserTokenSet, error) {
	refreshToken, err := s.loadRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return nil, err
	}
	sess, err := s.loadActiveSession(ctx, refreshToken.SessionID)
	if err != nil {
		return nil, err
	}
	if err := s.requireAdmission(ctx, refreshToken); err != nil {
		return nil, err
	}
	if err := s.ensureRefreshTokenUsable(ctx, refreshTokenValue, refreshToken); err != nil {
		return nil, err
	}

	principal := s.principalFromRefreshToken(refreshToken)
	newTokenSet, err := s.issueRotatedTokenSet(ctx, principal, sess)
	if err != nil {
		return nil, err
	}
	if err := s.sessionExtender.ExtendToRefreshExpiry(ctx, sess, newTokenSet.RefreshToken.ExpiresAt); err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) || perrors.IsCode(err, code.ErrInvalidArgument) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to extend session ttl")
	}

	rotated, err := s.tokenStore.RotateRefreshToken(ctx, refreshTokenValue, refreshToken.ID, newTokenSet.RefreshToken)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to rotate refresh token")
	}
	if !rotated {
		logger.L(ctx).Infow("refresh token rotation rejected because the old token was already consumed",
			"action", logger.ActionRefresh,
			"resource", logger.ResourceToken,
			"token_type", "refresh",
			"result", "conflict",
		)
		if err := s.revokeReplaySession(ctx, refreshToken.SessionID, refreshToken.UserID.String()); err != nil {
			return nil, err
		}
		return nil, perrors.WithCode(code.ErrRefreshTokenNotFound, "refresh token not found")
	}
	return newTokenSet, nil
}

func (s *refresher) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load refresh token")
	}
	if refreshToken != nil && refreshToken.SessionID != "" {
		if err := s.sessionRevoker.Revoke(ctx, refreshToken.SessionID, "refresh_token_revoked", refreshToken.UserID.String()); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token session")
		}
	}
	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token")
	}
	return nil
}

func (s *refresher) loadRefreshToken(ctx context.Context, value string) (*RefreshToken, error) {
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, value)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "refresh token not found or invalid")
	}
	if refreshToken == nil {
		consumed, consumedErr := s.tokenStore.GetConsumedRefreshToken(ctx, value)
		if consumedErr != nil {
			return nil, perrors.WrapC(consumedErr, code.ErrInternalServerError, "failed to inspect consumed refresh token")
		}
		if consumed != nil {
			if err := s.revokeReplaySession(ctx, consumed.SessionID, consumed.UserID.String()); err != nil {
				return nil, err
			}
		}
		return nil, perrors.WithCode(code.ErrRefreshTokenNotFound, "refresh token not found")
	}
	return refreshToken, nil
}

func (s *refresher) revokeReplaySession(ctx context.Context, sessionID, userID string) error {
	if sessionID == "" {
		return perrors.WithCode(code.ErrInternalServerError, "consumed refresh token has no session")
	}
	if err := s.sessionRevoker.Revoke(ctx, sessionID, "refresh_token_replay", userID); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke replayed refresh token session")
	}
	logger.L(ctx).Infow("refresh token replay revoked session",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceSession,
		"result", "success",
	)
	return nil
}

func (s *refresher) loadActiveSession(ctx context.Context, sessionID string) (*sessiondomain.Session, error) {
	sess, err := s.sessionLoader.GetActive(ctx, sessionID)
	if err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	return sess, nil
}

func (s *refresher) requireAdmission(ctx context.Context, refreshToken *RefreshToken) error {
	return admissiondomain.Require(ctx, s.admissionPolicy, admissiondomain.Subject{
		UserID: refreshToken.UserID, LoginIdentityID: refreshToken.LoginIdentityID,
	})
}

func (s *refresher) ensureRefreshTokenUsable(ctx context.Context, value string, refreshToken *RefreshToken) error {
	if !refreshToken.IsExpired() {
		return nil
	}
	_ = s.tokenStore.DeleteRefreshToken(ctx, value)
	return perrors.WithCode(code.ErrRefreshTokenExpired, "refresh token has expired")
}

func (s *refresher) principalFromRefreshToken(refreshToken *RefreshToken) *authentication.Principal {
	amr := refreshToken.AMR
	if len(amr) == 0 {
		amr = []string{"jwt"}
	}
	claims := s.refreshClaimsCodec.Decode(refreshToken.SessionClaims)
	if claims == nil {
		claims = make(map[string]any)
	}
	return &authentication.Principal{
		UserID: refreshToken.UserID, LoginIdentityID: refreshToken.LoginIdentityID,
		TenantID: refreshToken.TenantID, SessionID: refreshToken.SessionID,
		AuthMethod: refreshToken.AuthMethod, Realm: refreshToken.Realm, AMR: amr, Claims: claims,
	}
}

func (s *refresher) issueRotatedTokenSet(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*UserTokenSet, error) {
	if s.tokenSetMinter == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token set minter is not configured")
	}
	set, err := s.tokenSetMinter.MintTokenSet(ctx, principal, sess)
	if err != nil {
		return nil, err
	}
	if set == nil || set.AccessToken == nil || set.RefreshToken == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token set minter returned incomplete token set")
	}
	return set, nil
}
