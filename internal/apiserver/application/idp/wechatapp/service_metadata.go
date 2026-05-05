package wechatapp

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type wechatAppApplicationService struct {
	repo    domain.Repository
	creator domain.Creator
	rotater domain.CredentialRotater
}

// NewWechatAppApplicationService 创建微信应用管理应用服务。
func NewWechatAppApplicationService(
	repo domain.Repository,
	creator domain.Creator,
	rotater domain.CredentialRotater,
) WechatAppApplicationService {
	return &wechatAppApplicationService{
		repo:    repo,
		creator: creator,
		rotater: rotater,
	}
}

func (s *wechatAppApplicationService) CreateApp(ctx context.Context, dto CreateWechatAppDTO) (*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("创建微信应用",
		"action", logger.ActionCreate,
		"resource", "wechat_app",
		"app_id", dto.AppID,
		"name", dto.Name,
		"type", dto.Type,
	)

	app, err := s.creator.Create(ctx, dto.AppID, dto.Name, dto.Type)
	if err != nil {
		l.Errorw("创建微信应用实体失败",
			"action", logger.ActionCreate,
			"resource", "wechat_app",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to create wechat app: %w", err)
	}

	app.ID = meta.FromUint64(idutil.GetIntID())
	app.Cred = &domain.Credentials{}

	if dto.AppSecret != "" {
		if err := s.rotater.RotateAuthSecret(ctx, app, dto.AppSecret); err != nil {
			l.Errorw("设置认证密钥失败",
				"action", logger.ActionCreate,
				"resource", "wechat_app",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return nil, fmt.Errorf("failed to set auth secret: %w", err)
		}
	}

	if err := s.repo.Create(ctx, app); err != nil {
		l.Errorw("持久化微信应用失败",
			"action", logger.ActionCreate,
			"resource", "wechat_app",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to persist wechat app: %w", err)
	}

	l.Debugw("创建微信应用成功",
		"action", logger.ActionCreate,
		"resource", "wechat_app",
		"app_id", dto.AppID,
		"internal_id", app.ID.String(),
		"result", logger.ResultSuccess,
	)
	return toWechatAppResult(app), nil
}

func (s *wechatAppApplicationService) GetApp(ctx context.Context, appID string) (*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询微信应用",
		"action", logger.ActionRead,
		"resource", "wechat_app",
		"app_id", appID,
	)

	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		l.Errorw("查询微信应用失败",
			"action", logger.ActionRead,
			"resource", "wechat_app",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionRead,
			"resource", "wechat_app",
			"app_id", appID,
		)
		return nil, perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}

	l.Debugw("查询微信应用成功",
		"action", logger.ActionRead,
		"resource", "wechat_app",
		"app_id", appID,
		"result", logger.ResultSuccess,
	)
	return toWechatAppResult(app), nil
}

func (s *wechatAppApplicationService) ListApps(ctx context.Context, filter ListWechatAppsFilter) ([]*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("列出微信应用",
		"action", logger.ActionRead,
		"resource", "wechat_app",
	)

	apps, err := s.repo.List(ctx, domain.ListFilter{
		Type:   filter.Type,
		Status: filter.Status,
	})
	if err != nil {
		l.Errorw("列出微信应用失败",
			"action", logger.ActionRead,
			"resource", "wechat_app",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to list wechat apps: %w", err)
	}

	results := make([]*WechatAppResult, 0, len(apps))
	for _, app := range apps {
		results = append(results, toWechatAppResult(app))
	}
	return results, nil
}

func (s *wechatAppApplicationService) UpdateApp(ctx context.Context, appID string, dto UpdateWechatAppDTO) (*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("更新微信应用基础信息",
		"action", logger.ActionUpdate,
		"resource", "wechat_app",
		"app_id", appID,
	)

	if dto.Name == nil && dto.Type == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "at least one field must be updated")
	}

	app, err := s.getAppForMutation(ctx, appID)
	if err != nil {
		return nil, err
	}

	if dto.Name != nil {
		name := strings.TrimSpace(*dto.Name)
		if name == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "name cannot be empty")
		}
		app.Name = name
	}
	if dto.Type != nil {
		app.Type = *dto.Type
	}

	if err := s.repo.Update(ctx, app); err != nil {
		l.Errorw("更新微信应用失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app",
			"app_id", appID,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to update wechat app: %w", err)
	}
	return toWechatAppResult(app), nil
}

func (s *wechatAppApplicationService) EnableApp(ctx context.Context, appID string) (*WechatAppResult, error) {
	return s.changeAppStatus(ctx, appID, domain.StatusEnabled)
}

func (s *wechatAppApplicationService) DisableApp(ctx context.Context, appID string) (*WechatAppResult, error) {
	return s.changeAppStatus(ctx, appID, domain.StatusDisabled)
}

func (s *wechatAppApplicationService) getAppForMutation(ctx context.Context, appID string) (*domain.WechatApp, error) {
	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		return nil, perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}
	return app, nil
}

func (s *wechatAppApplicationService) changeAppStatus(ctx context.Context, appID string, status domain.Status) (*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("切换微信应用状态",
		"action", logger.ActionUpdate,
		"resource", "wechat_app",
		"app_id", appID,
		"status", status,
	)

	app, err := s.getAppForMutation(ctx, appID)
	if err != nil {
		return nil, err
	}

	switch status {
	case domain.StatusEnabled:
		app.Enable()
	case domain.StatusDisabled:
		app.Disable()
	default:
		return nil, perrors.WithCode(code.ErrWechatAppStatusInvalid, "unsupported app status: %s", status)
	}

	if err := s.repo.Update(ctx, app); err != nil {
		l.Errorw("切换微信应用状态失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app",
			"app_id", appID,
			"status", status,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, fmt.Errorf("failed to update wechat app status: %w", err)
	}
	return toWechatAppResult(app), nil
}
