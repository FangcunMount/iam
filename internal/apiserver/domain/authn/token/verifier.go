package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type verifier struct {
	tokenCodec      AccessTokenCodec
	tokenStore      Store
	sessionLoader   SessionLoader
	admissionPolicy AdmissionPolicy
}

// 实现 Verifier 接口
var _ Verifier = &verifier{}

func newVerifier(tokenCodec AccessTokenCodec, tokenStore Store, sessionLoader SessionLoader, admissionPolicy AdmissionPolicy) Verifier {
	return &verifier{
		tokenCodec: tokenCodec, tokenStore: tokenStore,
		sessionLoader: sessionLoader, admissionPolicy: admissionPolicy,
	}
}

func (s *verifier) VerifyToken(ctx context.Context, tokenValue string) (*TokenClaims, error) {
	// 解析令牌
	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse access token")
	}
	// 如果是服务令牌，则直接返回
	if claims.TokenType == TokenTypeService {
		return claims, nil
	}

	// 检查令牌是否被撤销
	if err := s.checkTokenValid(ctx, claims); err != nil {
		return nil, err
	}
	// 检查会话是否活跃
	if err := s.checkSessionActive(ctx, claims.SessionID); err != nil {
		return nil, err
	}
	// 检查准入策略
	if err := s.requireAdmission(ctx, claims.UserID, claims.LoginIdentityID); err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *verifier) checkTokenValid(ctx context.Context, claims *TokenClaims) error {
	isRevoked, err := s.tokenStore.IsAccessTokenRevoked(ctx, claims.TokenID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to check revoked access token")
	}
	if isRevoked {
		return perrors.WithCode(code.ErrTokenInvalid, "access token has been revoked")
	}
	return nil
}

func (s *verifier) checkSessionActive(ctx context.Context, sessionID string) error {
	_, err := s.sessionLoader.GetActive(ctx, sessionID)
	if err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) {
			return err
		}
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	return nil
}

func (s *verifier) requireAdmission(ctx context.Context, userID, loginIdentityID meta.ID) error {
	return admissiondomain.Require(ctx, s.admissionPolicy, admissiondomain.Subject{
		UserID: userID, LoginIdentityID: loginIdentityID,
	})
}
