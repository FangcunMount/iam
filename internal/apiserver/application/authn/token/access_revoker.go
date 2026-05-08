package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// accessTokenRevoker 撤销访问令牌
type accessTokenRevoker struct {
	tokenCodec     AccessTokenCodec
	tokenStore     Store
	sessionManager SessionManager
}

// 确保 accessTokenRevoker 实现 AccessRevoker 接口
var _ AccessRevoker = (*accessTokenRevoker)(nil)

// NewAccessTokenRevoker 创建 accessTokenRevoker
func newAccessTokenRevoker(tokenCodec AccessTokenCodec, tokenStore Store, sessionManager SessionManager) *accessTokenRevoker {
	return &accessTokenRevoker{
		tokenCodec:     tokenCodec,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
	}
}

// RevokeAccessToken 撤销访问令牌
func (s *accessTokenRevoker) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	// 验证访问令牌
	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse token for revocation")
	}
	// 如果访问令牌已过期，则直接返回
	if claims.IsExpired() {
		return nil
	}

	// 计算访问令牌剩余有效期
	expiry := time.Until(claims.ExpiresAt)
	if expiry <= 0 {
		return nil
	}
	// 标记访问令牌已撤销
	if err := s.tokenStore.MarkAccessTokenRevoked(ctx, claims.TokenID, expiry); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to mark access token revoked")
	}
	// 如果访问令牌有会话ID，则撤销会话
	if claims.SessionID != "" {
		if err := s.sessionManager.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}
	return nil
}
