package suggest

import (
	"context"
	"errors"
	"fmt"

	"github.com/robfig/cron/v3"

	"github.com/FangcunMount/component-base/pkg/log"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	mysqlsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/suggest"
	suggestauthz "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/authorization"
	suggestmemory "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/index/memory"
	suggestmetrics "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/metrics"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
	suggestvisibility "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/visibility"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

// SuggestModule 联想搜索模块
type SuggestModule struct {
	querier     appquery.Querier
	refresher   *apprefresh.Refresher
	rateLimiter suggestratelimit.Limiter
	cron        *cron.Cron

	config ModuleConfig
	cancel context.CancelFunc
}

// NewSuggestModule 创建模块
func NewSuggestModule() *SuggestModule {
	return &SuggestModule{}
}

// InitializeWithDeps 初始化联想模块。
func (m *SuggestModule) InitializeWithDeps(deps SuggestModuleDeps) error {
	cfg := deps.Config
	m.config = cfg

	if !cfg.Enable {
		log.Info("Suggest module disabled by config, skipping initialization")
		return nil
	}

	if deps.DB == nil {
		return fmt.Errorf("suggest module requires mysql connection")
	}
	if deps.Environment == genericapiserver.EnvironmentProduction && cfg.Query.DisableMobileMask {
		return fmt.Errorf("suggest.disable_mobile_mask is forbidden in production")
	}

	var visibility appquery.VisibilityReader = mysqlsuggest.NewVisibilityReader(deps.DB)
	if ttl := cfg.Visibility.CacheTTL(); ttl > 0 {
		visibility = suggestvisibility.NewCachedReader(visibility, ttl)
	}

	factsReader := suggestauthz.NewFactsReader(deps.RoutePermissionChecker)
	scopeResolver := appquery.NewScopeResolver(factsReader, visibility)

	runtime := suggestmemory.NewRuntime(cfg.Memory, metrics)
	metrics := suggestmetrics.Recorder{}

	querySvc := appquery.NewService(cfg.Query, scopeResolver, runtime, metrics)
	m.querier = querySvc

	loader := mysqlsuggest.NewLoader(deps.DB, cfg.Loader)
	m.refresher = apprefresh.NewRefresher(loader, runtime, metrics)
	m.rateLimiter = suggestratelimit.NewFromConfig(cfg.RateLimit, deps.RedisClient)

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.startRefresher(ctx, cfg); err != nil {
		cancel()
		if cfg.Required {
			return fmt.Errorf("start suggest refresher: %w", err)
		}
		log.Errorw("suggest module degraded", "error", err)
		m.querier = appquery.DegradedQuerier{}
		return nil
	}
	m.cancel = cancel

	log.Info("✅ Suggest module initialized")
	return nil
}

// Cleanup 停止调度
func (m *SuggestModule) Cleanup() error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.cron != nil {
		ctx := m.cron.Stop()
		<-ctx.Done()
		m.cron = nil
	}
	return nil
}

// CheckHealth 检查是否已加载数据
func (m *SuggestModule) CheckHealth() error {
	if !m.config.Enable {
		return nil
	}
	if m.querier == nil {
		return fmt.Errorf("suggest service not initialized")
	}
	if m.refresher == nil || !m.refresher.HasSuccessfulRefresh() {
		return fmt.Errorf("suggest full refresh has not completed")
	}
	return nil
}

func (m *SuggestModule) IsInitialized() bool {
	return m != nil && m.querier != nil
}

func (m *SuggestModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		Querier:     m.querier,
		Metrics:     suggestmetrics.Recorder{},
		RateLimiter: m.rateLimiter,
	}
}

func (m *SuggestModule) RuntimeCapabilities() RuntimeCapabilities {
	if m == nil {
		return RuntimeCapabilities{}
	}
	return RuntimeCapabilities{Cleanup: m.Cleanup}
}

func (m *SuggestModule) startRefresher(ctx context.Context, cfg ModuleConfig) error {
	if m.refresher == nil {
		return fmt.Errorf("suggest refresher not initialized")
	}
	if err := m.refresher.RunFull(ctx); err != nil {
		return err
	}

	m.cron = cron.New()
	if _, err := m.cron.AddFunc(cfg.FullSyncCron, func() {
		if err := m.refresher.RunFull(ctx); err != nil {
			if errors.Is(err, apprefresh.ErrRefreshInProgress) {
				log.Info("suggest full sync skipped: refresh already in progress")
				return
			}
			log.Errorw("suggest full sync failed", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("add suggest full cron failed: %w", err)
	}
	if cfg.DeltaSyncCron != "" {
		if _, err := m.cron.AddFunc(cfg.DeltaSyncCron, func() {
			if err := m.refresher.RunDelta(ctx); err != nil {
				if errors.Is(err, apprefresh.ErrRefreshInProgress) {
					log.Info("suggest delta sync skipped: refresh already in progress")
					return
				}
				log.Errorw("suggest delta sync failed", "error", err)
			}
		}); err != nil {
			return fmt.Errorf("add suggest delta cron failed: %w", err)
		}
	}
	m.cron.Start()
	return nil
}
