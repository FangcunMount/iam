package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissionapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ====================================================
// ================== Implementation ==================
// ====================================================
// verifier 验证访问令牌
type verifier struct {
	tokenCodec      AccessTokenCodec // 令牌编码器
	tokenStore      Store            // 令牌存储
	sessionLoader   SessionLoader    // 会话加载器
	admissionPolicy AdmissionPolicy  // 认证准入策略
}

// 确保 verifier 实现 verifierPort 接口。
var _ verifierPort = (*verifier)(nil)

// newVerifier 创建 verifier。
func newVerifier(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionLoader SessionLoader,
	admissionPolicy AdmissionPolicy,
) verifierPort {
	return &verifier{
		tokenCodec:      tokenCodec,
		tokenStore:      tokenStore,
		sessionLoader:   sessionLoader,
		admissionPolicy: admissionPolicy,
	}
}

// VerifyToken 验证 JWT bearer（用户 access 或 service access）。
func (s *verifier) VerifyToken(ctx context.Context, tokenValue string) (*TokenClaims, error) {
	// 解析访问令牌
	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse access token")
	}
	// 如果令牌类型为服务令牌，则直接返回
	if claims.TokenType == TokenTypeService {
		return claims, nil
	}

	// 检查令牌合法性
	if err := s.checkTokenValid(ctx, claims); err != nil {
		return nil, err
	}

	// 检查会话是否活跃
	if err := s.checkSessionActive(ctx, claims.SessionID); err != nil {
		return nil, err
	}

	// 检查认证主体准入状态
	if err := s.requireAdmission(ctx, claims.UserID, claims.LoginIdentityID); err != nil {
		return nil, err
	}

	return claims, nil
}

// checkTokenValid 检查令牌合法性
func (s *verifier) checkTokenValid(ctx context.Context, claims *TokenClaims) error {
	// 检查令牌是否已撤销
	isRevoked, err := s.tokenStore.IsAccessTokenRevoked(ctx, claims.TokenID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to check revoked access token")
	}
	if isRevoked {
		return perrors.WithCode(code.ErrTokenInvalid, "access token has been revoked")
	}
	return nil
}

// checkSessionActive 检查会话是否活跃
func (s *verifier) checkSessionActive(ctx context.Context, sessionID string) error {
	// 加载活跃会话
	_, err := s.sessionLoader.GetActive(ctx, sessionID)
	if err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) {
			return err
		}
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	return nil
}

// requireAdmission 检查认证主体准入状态。
func (s *verifier) requireAdmission(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) error {
	return admissionapp.Require(ctx, s.admissionPolicy, userID, loginIdentityID)
}
