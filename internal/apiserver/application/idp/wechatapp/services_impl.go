package wechatapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= 应用服务实现 =============

// ================================================
// ==== WechatAppApplicationService 实现 =====
// ================================================

type wechatAppApplicationService struct {
	repo    domain.Repository
	creator domain.Creator
	rotater domain.CredentialRotater
}

// NewWechatAppApplicationService 创建微信应用管理应用服务
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

// CreateApp 创建微信应用
func (s *wechatAppApplicationService) CreateApp(ctx context.Context, dto CreateWechatAppDTO) (*WechatAppResult, error) {
	l := logger.L(ctx)
	l.Debugw("创建微信应用",
		"action", logger.ActionCreate,
		"resource", "wechat_app",
		"app_id", dto.AppID,
		"name", dto.Name,
		"type", dto.Type,
	)

	// 调用领域服务创建微信应用实体
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

	// 分配内部 ID
	id := meta.FromUint64(idutil.GetIntID()) // 新生成的 ID 必定有效
	app.ID = id

	// 初始化凭据结构
	app.Cred = &domain.Credentials{}

	// 如果提供了 AppSecret，设置认证密钥
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

	// 持久化
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

	// 转换为结果 DTO
	return toWechatAppResult(app), nil
}

// GetApp 查询微信应用
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

// ListApps 列出微信应用。
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

// UpdateApp 更新微信应用基础信息。
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

// EnableApp 启用微信应用。
func (s *wechatAppApplicationService) EnableApp(ctx context.Context, appID string) (*WechatAppResult, error) {
	return s.changeAppStatus(ctx, appID, domain.StatusEnabled)
}

// DisableApp 禁用微信应用。
func (s *wechatAppApplicationService) DisableApp(ctx context.Context, appID string) (*WechatAppResult, error) {
	return s.changeAppStatus(ctx, appID, domain.StatusDisabled)
}

// =========================================================
// ==== WechatAppCredentialApplicationService 实现 =====
// =========================================================

type wechatAppCredentialApplicationService struct {
	repo    domain.Repository
	rotater domain.CredentialRotater
}

// NewWechatAppCredentialApplicationService 创建微信应用凭据应用服务
func NewWechatAppCredentialApplicationService(
	repo domain.Repository,
	rotater domain.CredentialRotater,
) WechatAppCredentialApplicationService {
	return &wechatAppCredentialApplicationService{
		repo:    repo,
		rotater: rotater,
	}
}

// RotateAuthSecret 轮换认证密钥（AppSecret）
func (s *wechatAppCredentialApplicationService) RotateAuthSecret(ctx context.Context, appID string, newSecret string) error {
	l := logger.L(ctx)
	l.Debugw("轮换认证密钥",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
	)

	// 查询应用
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

	// 确保凭据结构已初始化
	if app.Cred == nil {
		app.Cred = &domain.Credentials{}
	}

	// 调用领域服务轮换密钥
	if err := s.rotater.RotateAuthSecret(ctx, app, newSecret); err != nil {
		l.Errorw("轮换认证密钥失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to rotate auth secret: %w", err)
	}

	// 持久化
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

// RotateMsgSecret 轮换消息加解密密钥
func (s *wechatAppCredentialApplicationService) RotateMsgSecret(ctx context.Context, appID string, callbackToken string, encodingAESKey string) error {
	l := logger.L(ctx)
	l.Debugw("轮换消息加解密密钥",
		"action", logger.ActionUpdate,
		"resource", "wechat_app_credential",
		"app_id", appID,
	)

	// 查询应用
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

	// 确保凭据结构已初始化
	if app.Cred == nil {
		app.Cred = &domain.Credentials{}
	}

	// 调用领域服务轮换密钥
	if err := s.rotater.RotateMsgAESKey(ctx, app, callbackToken, encodingAESKey); err != nil {
		l.Errorw("轮换消息密钥失败",
			"action", logger.ActionUpdate,
			"resource", "wechat_app_credential",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return fmt.Errorf("failed to rotate msg secret: %w", err)
	}

	// 持久化
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

// ======================================================
// ==== WechatAppTokenApplicationService 实现 =====
// ======================================================

type wechatAppTokenApplicationService struct {
	repo          domain.Repository
	tokenCacher   domain.AccessTokenCacher
	tokenProvider domain.AppTokenProvider
	cache         domain.AccessTokenCache
}

// NewWechatAppTokenApplicationService 创建微信应用访问令牌应用服务
func NewWechatAppTokenApplicationService(
	repo domain.Repository,
	tokenCacher domain.AccessTokenCacher,
	tokenProvider domain.AppTokenProvider,
	cache domain.AccessTokenCache,
) WechatAppTokenApplicationService {
	return &wechatAppTokenApplicationService{
		repo:          repo,
		tokenCacher:   tokenCacher,
		tokenProvider: tokenProvider,
		cache:         cache,
	}
}

// GetAccessToken 获取访问令牌（带缓存和自动刷新）
func (s *wechatAppTokenApplicationService) GetAccessToken(ctx context.Context, appID string) (string, error) {
	// 查询应用
	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		return "", fmt.Errorf("wechat app not found: %s", appID)
	}

	// 使用缓存器获取令牌（自动处理刷新）
	token, err := s.tokenCacher.EnsureToken(ctx, app, 120*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return token, nil
}

// RefreshAccessToken 强制刷新访问令牌
func (s *wechatAppTokenApplicationService) RefreshAccessToken(ctx context.Context, appID string) (string, error) {
	// 查询应用
	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		return "", fmt.Errorf("wechat app not found: %s", appID)
	}

	// 强制刷新：获取新令牌
	aat, err := s.tokenProvider.Fetch(ctx, app)
	if err != nil {
		return "", fmt.Errorf("failed to fetch access token: %w", err)
	}

	// 更新缓存
	ttl := time.Until(aat.ExpiresAt)
	if ttl < 60*time.Second {
		ttl = 60 * time.Second
	}
	if err := s.cache.Set(ctx, appID, aat, ttl); err != nil {
		return "", fmt.Errorf("failed to cache access token: %w", err)
	}

	return aat.Token, nil
}

// ============= 辅助函数 =============

// toWechatAppResult 转换领域对象为结果 DTO
func toWechatAppResult(app *domain.WechatApp) *WechatAppResult {
	if app == nil {
		return nil
	}

	return &WechatAppResult{
		ID:     app.ID.String(),
		AppID:  app.AppID,
		Name:   app.Name,
		Type:   app.Type,
		Status: app.Status,
	}
}
