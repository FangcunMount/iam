package token

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
)

// IssueServiceToken 签发服务间访问令牌。
func (s *tokenApplicationService) IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error) {
	l := logger.L(ctx)

	l.Debugw("开始签发服务令牌",
		"action", logger.ActionCreate,
		"resource", logger.ResourceToken,
		"token_type", "service",
		"subject", req.Subject,
		"audience", req.Audience,
	)

	tokenPair, err := s.serviceTokenIssuer.IssueServiceToken(ctx, req.Subject, req.Audience, req.Attributes, req.TTL)
	if err != nil {
		l.Warnw("签发服务令牌失败",
			"action", logger.ActionCreate,
			"resource", logger.ResourceToken,
			"token_type", "service",
			"subject", req.Subject,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	l.Debugw("服务令牌签发成功",
		"action", logger.ActionCreate,
		"resource", logger.ResourceToken,
		"token_type", "service",
		"subject", req.Subject,
		"result", logger.ResultSuccess,
	)

	return &TokenIssueResult{TokenPair: tokenPair}, nil
}
