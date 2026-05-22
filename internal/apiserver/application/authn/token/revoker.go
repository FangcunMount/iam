package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// revoker 用于撤销令牌和关联会话。
type revoker struct {
	tokenCodec     AccessTokenCodec
	tokenStore     Store
	sessionRevoker SessionRevoker
}

// 确保 revoker 实现 revokerPort 接口。
var _ revokerPort = (*revoker)(nil)

// newRevoker 创建 revoker。
func newRevoker(tokenCodec AccessTokenCodec, tokenStore Store, sessionRevoker SessionRevoker) *revoker {
	return &revoker{
		tokenCodec:     tokenCodec,
		tokenStore:     tokenStore,
		sessionRevoker: sessionRevoker,
	}
}

// RevokeBearerToken 撤销 JWT bearer。
func (s *revoker) RevokeBearerToken(ctx context.Context, tokenValue string) error {
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
		if err := s.sessionRevoker.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}
	return nil
}
