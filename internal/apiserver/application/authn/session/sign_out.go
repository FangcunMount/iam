package session

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// SignOut 编排登出：撤销调用者提供的访问令牌或刷新令牌。
type SignOut struct {
	tokenService tokenapp.TokenApplicationService
}

// Execute 执行登出。
// 参数：ctx 上下文, cmd 登出命令
// 返回：错误
// 职责：执行登出逻辑，返回错误
func (s *SignOut) Execute(ctx context.Context, cmd SignOutCommand) error {
	// 确保日志记录器已准备好
	l := logger.L(ctx)

	if err := validateSignOutCommand(cmd); err != nil {
		l.Warnw("登出请求缺少令牌",
			"action", logger.ActionLogout,
			"result", logger.ResultFailed,
		)
		return err
	}

	// 确保依赖已准备好
	if err := s.ensureReady(); err != nil {
		return err
	}

	// 撤销刷新令牌
	if refreshToken := tokenString(cmd.RefreshToken); refreshToken != "" {
		if err := s.revokeRefreshToken(ctx, refreshToken); err != nil {
			return err
		}
	}

	// 撤销访问令牌
	if accessToken := tokenString(cmd.AccessToken); accessToken != "" {
		if err := s.revokeAccessToken(ctx, accessToken); err != nil {
			return err
		}
	}

	// 记录登出成功
	l.Debugw("当前登录会话已退出",
		"action", logger.ActionLogout,
		"resource", "session",
		"result", logger.ResultSuccess,
	)
	return nil
}

// validateSignOutCommand 验证登出命令
// 参数：cmd 登出命令
// 返回：错误
// 职责：验证登出命令，返回错误
func validateSignOutCommand(cmd SignOutCommand) error {
	if tokenString(cmd.AccessToken) == "" && tokenString(cmd.RefreshToken) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "either access_token or refresh_token is required")
	}
	return nil
}

// ensureReady 确保依赖已准备好
// 参数：s SignOut 用例
// 返回：错误
// 职责：确保依赖已准备好，返回错误
func (s *SignOut) ensureReady() error {
	if s == nil || s.tokenService == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "token service is not initialized")
	}
	return nil
}

// revokeRefreshToken 撤销刷新令牌
// 参数：ctx 上下文, refreshToken 刷新令牌
// 返回：错误
// 职责：撤销刷新令牌，返回错误
func (s *SignOut) revokeRefreshToken(ctx context.Context, refreshToken string) error {
	l := logger.L(ctx)

	// 撤销刷新令牌
	if err := s.tokenService.RevokeRefreshToken(ctx, refreshToken); err != nil {
		l.Errorw("撤销刷新令牌失败",
			"action", logger.ActionLogout,
			"error_category", "session_store",
			"retryable", true,
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrTokenRevokeFailed, "failed to revoke refresh token")
	}

	// 记录登出成功
	l.Debugw("刷新令牌已撤销",
		"action", logger.ActionLogout,
		"result", logger.ResultSuccess,
	)
	return nil
}

// revokeAccessToken 撤销访问令牌
// 参数：ctx 上下文, accessToken 访问令牌
// 返回：错误
// 职责：撤销访问令牌，返回错误
func (s *SignOut) revokeAccessToken(ctx context.Context, accessToken string) error {
	l := logger.L(ctx)

	// 撤销访问令牌
	if err := s.tokenService.RevokeAccessToken(ctx, accessToken); err != nil {
		l.Errorw("撤销访问令牌失败",
			"action", logger.ActionLogout,
			"error_category", "token_store",
			"retryable", true,
			"result", logger.ResultFailed,
		)
		return perrors.WithCode(code.ErrTokenRevokeFailed, "failed to revoke access token")
	}

	// 记录登出成功
	l.Debugw("访问令牌已撤销",
		"action", logger.ActionLogout,
		"result", logger.ResultSuccess,
	)
	return nil
}

// tokenString 转换令牌字符串
// 参数：value 令牌值
// 返回：令牌字符串
// 职责：转换令牌字符串，返回令牌字符串
func tokenString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
