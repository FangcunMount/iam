package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/security/sanitize"
)

// RevokeAccessToken 撤销访问令牌
func (s *tokenApplicationService) RevokeAccessToken(ctx context.Context, accessToken string) error {
	l := logger.L(ctx)

	l.Debugw("开始撤销访问令牌",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_hint", sanitize.MaskToken(accessToken),
	)

	err := s.accessRevoker.RevokeAccessToken(ctx, accessToken)
	if err != nil {
		l.Errorw("撤销访问令牌失败",
			"action", logger.ActionRevoke,
			"resource", logger.ResourceToken,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke token: %v", err)
	}

	l.Debugw("访问令牌撤销成功",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"result", logger.ResultSuccess,
	)
	return nil
}

// RevokeRefreshToken 撤销刷新令牌
func (s *tokenApplicationService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	l := logger.L(ctx)

	l.Debugw("开始撤销刷新令牌",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_type", "refresh",
		"token_hint", sanitize.MaskToken(refreshToken),
	)

	err := s.tokenRefresher.RevokeRefreshToken(ctx, refreshToken)
	if err != nil {
		l.Errorw("撤销刷新令牌失败",
			"action", logger.ActionRevoke,
			"resource", logger.ResourceToken,
			"token_type", "refresh",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "failed to revoke refresh token: %v", err)
	}

	l.Debugw("刷新令牌撤销成功",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceToken,
		"token_type", "refresh",
		"result", logger.ResultSuccess,
	)
	return nil
}
