package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ====================================================
// ================== Driving Ports ===================
// ====================================================
// serviceTokenIssuerPort 签发服务间访问令牌；服务令牌不创建 session 或 refresh token。
//
// 返回值必须包含 access token 和 refresh token。
type serviceTokenIssuerPort interface {
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)
}

// ====================================================
// ================== Implementation ==================
// ====================================================

// serviceTokenIssuer 签发服务令牌
type serviceTokenIssuer struct {
	tokenCodec AccessTokenCodec
	accessTTL  time.Duration
}

// 确保 serviceTokenIssuer 实现 serviceTokenIssuerPort 接口。
var _ serviceTokenIssuerPort = (*serviceTokenIssuer)(nil)

// newServiceTokenIssuer 创建 serviceTokenIssuer。
func newServiceTokenIssuer(tokenCodec AccessTokenCodec, accessTTL time.Duration) *serviceTokenIssuer {
	return &serviceTokenIssuer{
		tokenCodec: tokenCodec,
		accessTTL:  accessTTL,
	}
}

// IssueServiceToken 签发服务令牌。
func (s *serviceTokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	// 如果主题为空，则返回错误
	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	// 如果有效期小于等于0，则使用默认有效期
	if ttl <= 0 {
		ttl = s.accessTTL
	}
	// 颁发服务令牌
	serviceToken, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return NewTokenPair(serviceToken, nil), nil
}
