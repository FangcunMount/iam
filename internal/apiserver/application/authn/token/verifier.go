package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ====================================================
// ================== Driving Ports ===================
// ====================================================
// verifierPort 在线验证 access token，
type verifierPort interface {
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// ====================================================
// ================== Implementation ==================
// ====================================================
// verifier 验证访问令牌
type verifier struct {
	tokenCodec     AccessTokenCodec       // 令牌编码器
	tokenStore     Store                  // 令牌存储
	sessionManager SessionManager         // 会话管理器
	accessChecker  SubjectAccessEvaluator // 主体访问状态评估器
}

// 确保 verifier 实现 verifierPort 接口。
var _ verifierPort = (*verifier)(nil)

// newVerifier 创建 verifier。
func newVerifier(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
) verifierPort {
	return &verifier{
		tokenCodec:     tokenCodec,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		accessChecker:  accessChecker,
	}
}

// VerifyAccessToken 验证访问令牌
// 职责：验证访问令牌是否有效：1. 令牌是否已撤销；2. 会话是否活跃；3. 主体访问权限是否允许。
// 返回值：访问令牌声明
func (s *verifier) VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error) {
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

	// 检查主体访问权限
	if err := s.checkSubjectAccessAllowed(ctx, claims.UserID, claims.LoginIdentityID); err != nil {
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
	// 加载会话
	sess, err := s.sessionManager.Get(ctx, sessionID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	// 检查会话是否活跃
	if sess == nil || !sess.IsActive() {
		return perrors.WithCode(code.ErrTokenInvalid, "session has been revoked or expired")
	}
	return nil
}

// checkSubjectAccessAllowed 检查主体访问权限
func (s *verifier) checkSubjectAccessAllowed(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) error {
	// 检查主体访问权限
	decision, err := s.accessChecker.Evaluate(ctx, userID, loginIdentityID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}

	// 检查主体访问权限是否允许
	if !decision.IsAllowed() {
		return subjectAccessVerifyError(decision.Status)
	}
	return nil
}

// subjectAccessError 转换主体访问状态为错误
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
