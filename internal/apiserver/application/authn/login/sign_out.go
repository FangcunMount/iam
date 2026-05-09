package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// SignOut 编排登出：撤销调用者提供的访问令牌或刷新令牌。
type SignOut struct {
	tokenService tokenapp.TokenApplicationService
}

func (s *SignOut) Execute(ctx context.Context, cmd SignOutCommand) error {
	l := logger.L(ctx)

	if (cmd.AccessToken == nil || *cmd.AccessToken == "") &&
		(cmd.RefreshToken == nil || *cmd.RefreshToken == "") {
		l.Warnw("登出请求缺少令牌",
			"action", logger.ActionLogout,
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrInvalidArgument, "either access_token or refresh_token is required")
	}

	if cmd.RefreshToken != nil && *cmd.RefreshToken != "" {
		if s == nil || s.tokenService == nil {
			return perrors.WithCode(code.ErrInvalidArgument, "token service is not initialized")
		}
		if err := s.tokenService.RevokeRefreshToken(ctx, *cmd.RefreshToken); err != nil {
			l.Errorw("撤销刷新令牌失败",
				"action", logger.ActionLogout,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return perrors.WithCode(code.ErrTokenRevokeFailed, "failed to revoke refresh token")
		}
		l.Debugw("刷新令牌已撤销",
			"action", logger.ActionLogout,
			"result", logger.ResultSuccess,
		)
	}

	if cmd.AccessToken != nil && *cmd.AccessToken != "" {
		if s == nil || s.tokenService == nil {
			return perrors.WithCode(code.ErrInvalidArgument, "token service is not initialized")
		}
		if err := s.tokenService.RevokeAccessToken(ctx, *cmd.AccessToken); err != nil {
			l.Errorw("撤销访问令牌失败",
				"action", logger.ActionLogout,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return perrors.WithCode(code.ErrTokenRevokeFailed, "failed to revoke access token")
		}
		l.Debugw("访问令牌已撤销",
			"action", logger.ActionLogout,
			"result", logger.ResultSuccess,
		)
	}

	l.Debugw("当前登录会话已退出",
		"action", logger.ActionLogout,
		"resource", "session",
		"result", logger.ResultSuccess,
	)
	return nil
}
