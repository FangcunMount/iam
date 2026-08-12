package wechatapp

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type wechatAppTokenApplicationService struct {
	repo          domain.Repository
	tokenCacher   domain.AccessTokenCacher
	tokenProvider domain.AppTokenProvider
	cache         domain.AccessTokenCache
}

// NewWechatAppTokenApplicationService 创建微信应用访问令牌应用服务。
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

func (s *wechatAppTokenApplicationService) GetAccessToken(ctx context.Context, appID string) (string, error) {
	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		return "", perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}

	token, err := s.tokenCacher.EnsureToken(ctx, app, 120*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	return token, nil
}

func (s *wechatAppTokenApplicationService) RefreshAccessToken(ctx context.Context, appID string) (string, error) {
	app, err := s.repo.GetByAppID(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("failed to query wechat app: %w", err)
	}
	if app == nil {
		return "", perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: %s", appID)
	}

	aat, err := s.tokenProvider.Fetch(ctx, app)
	if err != nil {
		return "", fmt.Errorf("failed to fetch access token: %w", err)
	}

	ttl := time.Until(aat.ExpiresAt)
	if ttl < 60*time.Second {
		ttl = 60 * time.Second
	}
	if err := s.cache.Set(ctx, appID, aat, ttl); err != nil {
		return "", fmt.Errorf("failed to cache access token: %w", err)
	}
	return aat.Token, nil
}
