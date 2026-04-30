package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"

	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

// RefreshToken 刷新访问令牌
func (s *tokenApplicationService) RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error) {
	l := logger.L(ctx)

	l.Debugw("开始刷新访问令牌",
		"action", logger.ActionRefresh,
		"resource", logger.ResourceToken,
		"token_hint", sanitize.MaskToken(refreshToken),
	)

	// 使用刷新令牌获取新的令牌对
	tokenPair, err := s.tokenRefresher.RefreshToken(ctx, refreshToken)
	if err != nil {
		l.Warnw("刷新令牌失败",
			"action", logger.ActionRefresh,
			"resource", logger.ResourceToken,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, perrors.WithCode(code.ErrTokenInvalid, "failed to refresh token: %v", err)
	}

	l.Debugw("访问令牌刷新成功",
		"action", logger.ActionRefresh,
		"resource", logger.ResourceToken,
		"result", logger.ResultSuccess,
	)

	return &TokenRefreshResult{
		TokenPair: tokenPair,
	}, nil
}
