package suggest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/FangcunMount/component-base/pkg/log"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	mysqlsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/suggest"
	suggestaccess "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/access"
	suggestmetrics "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/metrics"
	suggestratelimit "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/ratelimit"
	searchruntime "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/search"
)

// SuggestModule 联想搜索模块
type SuggestModule struct {
	service     appsuggest.ProfileSuggestor
	refresher   *appsuggest.ProfileIndexRefresher
	rateLimiter appsuggest.RateLimiter
	cron        *cron.Cron

	config appsuggest.Config
	cancel context.CancelFunc
}

// NewSuggestModule 创建模块
func NewSuggestModule() *SuggestModule {
	return &SuggestModule{}
}

// InitializeWithDeps 初始化联想模块。
func (m *SuggestModule) InitializeWithDeps(deps SuggestModuleDeps) error {
	cfg := deps.Config.WithDefaults()
	m.config = cfg

	if !cfg.Enable {
		log.Info("Suggest module disabled by config, skipping initialization")
		return nil
	}

	if deps.DB == nil {
		return fmt.Errorf("suggest module requires mysql connection")
	}
	if strings.EqualFold(strings.TrimSpace(deps.AppMode), "production") && cfg.DisableMobileMask {
		return fmt.Errorf("suggest.disable_mobile_mask is forbidden in production")
	}

	var visibility appsuggest.ProfileVisibilityIDsResolver = mysqlsuggest.NewProfileVisibilityResolver(deps.DB)
	if cfg.VisibilityCacheTTLSeconds > 0 {
		visibility = suggestaccess.NewCachedProfileVisibilityResolver(
			visibility,
			time.Duration(cfg.VisibilityCacheTTLSeconds)*time.Second,
		)
	}
	scopeProvider := suggestaccess.NewOperatingProfileAccessScopeProvider(deps.RouteAuthorization, visibility)
	runtime := searchruntime.NewRuntime()
	metrics := suggestmetrics.Recorder{}
	m.service = appsuggest.NewServiceWithRuntime(cfg, runtime, scopeProvider, metrics)

	loader := mysqlsuggest.NewLoader(deps.DB, mysqlsuggest.LoaderConfig{
		FullSQL:             cfg.FullSQL,
		DeltaSQL:            cfg.DeltaSQL,
		PlaceholderOrgID:    cfg.LoaderPlaceholderOrgID,
		PlaceholderTenantID: cfg.LoaderPlaceholderTenantID,
	})
	var snapshot appsuggest.SnapshotWriter
	if cfg.Snapshot {
		snapshot = searchruntime.NewFileSnapshotWriter(cfg.DataDir)
	}
	m.refresher = appsuggest.NewProfileIndexRefresher(loader, runtime, snapshot, metrics)
	m.rateLimiter = suggestratelimit.NewFromConfig(cfg.RateLimit, deps.RedisClient)

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.startRefresher(ctx, cfg); err != nil {
		cancel()
		if cfg.Required {
			return fmt.Errorf("start suggest refresher: %w", err)
		}
		log.Errorw("suggest module degraded", "error", err)
		m.service = appsuggest.DegradedService{}
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
	if m.service == nil {
		return fmt.Errorf("suggest service not initialized")
	}
	return nil
}

func (m *SuggestModule) IsInitialized() bool {
	return m != nil && m.service != nil
}

func (m *SuggestModule) ApplicationCapabilities() ApplicationCapabilities {
	if m == nil {
		return ApplicationCapabilities{}
	}
	return ApplicationCapabilities{
		Service:     m.service,
		RateLimit:   m.config.RateLimit,
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

func (m *SuggestModule) startRefresher(ctx context.Context, cfg appsuggest.Config) error {
	if m.refresher == nil {
		return fmt.Errorf("suggest refresher not initialized")
	}
	if err := m.refresher.RunFull(ctx); err != nil {
		return err
	}

	m.cron = cron.New()
	if _, err := m.cron.AddFunc(cfg.FullSyncCron, func() {
		if err := m.refresher.RunFull(ctx); err != nil {
			log.Errorw("suggest full sync failed", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("add suggest full cron failed: %w", err)
	}
	if cfg.DeltaSyncCron != "" {
		if _, err := m.cron.AddFunc(cfg.DeltaSyncCron, func() {
			if err := m.refresher.RunDelta(ctx); err != nil {
				log.Errorw("suggest delta sync failed", "error", err)
			}
		}); err != nil {
			return fmt.Errorf("add suggest delta cron failed: %w", err)
		}
	}
	m.cron.Start()
	return nil
}
