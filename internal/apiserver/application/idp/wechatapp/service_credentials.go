package wechatapp

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type wechatAppCredentialApplicationService struct {
	repo    domain.Repository
	rotater domain.CredentialRotater
}

// NewWechatAppCredentialApplicationService 创建微信应用凭据应用服务。
func NewWechatAppCredentialApplicationService(
	repo domain.Repository,
	rotater domain.CredentialRotater,
) WechatAppCredentialApplicationService {
	return &wechatAppCredentialApplicationService{
		repo:    repo,
		rotater: rotater,
	}
}

func (s *wechatAppCredentialApplicationService) RotateAuthSecret(ctx context.Context, appID string, newSecret string) error {
	l := logger.L(ctx)
	l.Debugw("轮换认证密钥",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
	)

	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		l.Errorw("查询微信应用失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"app_id", appID,
		)
		return perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}

	if app.Cred == nil {
		app.Cred = &domain.Credentials{}
	}
	if err := s.rotater.RotateAuthSecret(ctx, app, newSecret); err != nil {
		l.Errorw("轮换认证密钥失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to rotate auth secret: %w", err)
	}
	if err := s.repo.Update(ctx, app); err != nil {
		l.Errorw("持久化微信应用失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to update wechat app: %w", err)
	}

	l.Debugw("轮换认证密钥成功",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
		"result", logger.ResultSuccess,
	)
	return nil
}

func (s *wechatAppCredentialApplicationService) RotateMsgSecret(ctx context.Context, appID string, callbackToken string, encodingAESKey string) error {
	l := logger.L(ctx)
	l.Debugw("轮换消息加解密密钥",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
	)

	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		l.Errorw("查询微信应用失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"app_id", appID,
		)
		return perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}

	if app.Cred == nil {
		app.Cred = &domain.Credentials{}
	}
	if err := s.rotater.RotateMsgAESKey(ctx, app, callbackToken, encodingAESKey); err != nil {
		l.Errorw("轮换消息密钥失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to rotate msg secret: %w", err)
	}
	if err := s.repo.Update(ctx, app); err != nil {
		l.Errorw("持久化微信应用失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to update wechat app: %w", err)
	}

	l.Debugw("轮换消息加解密密钥成功",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
		"result", logger.ResultSuccess,
	)
	return nil
}
