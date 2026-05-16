package assembler

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	mysqlsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/suggest"
	suggestaccess "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/access"
	searchruntime "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/search"
	authn "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// SuggestModule 联想搜索模块
type SuggestModule struct {
	service   appsuggest.ProfileSuggestor
	refresher *appsuggest.ProfileIndexRefresher
	cron      *cron.Cron

	config appsuggest.Config
	cancel context.CancelFunc
}

// NewSuggestModule 创建模块
func NewSuggestModule() *SuggestModule {
	return &SuggestModule{}
}

// SuggestModuleDeps 初始化依赖
type SuggestModuleDeps struct {
	DB                 *gorm.DB
	Config             appsuggest.Config
	RouteAuthorization authn.RouteAuthorizationRuntime
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

	scopeProvider := suggestaccess.NewOperatingProfileAccessScopeProvider(deps.RouteAuthorization, nil)
	runtime := searchruntime.NewRuntime()
	m.service = appsuggest.NewServiceWithRuntime(cfg, runtime, scopeProvider)

	loader := mysqlsuggest.NewLoader(deps.DB, mysqlsuggest.LoaderConfig{
		FullSQL:             cfg.FullSQL,
		DeltaSQL:            cfg.DeltaSQL,
		PlaceholderTenantID: cfg.LoaderPlaceholderTenantID,
	})
	var snapshot appsuggest.SnapshotWriter
	if cfg.Snapshot {
		snapshot = searchruntime.NewFileSnapshotWriter(cfg.DataDir)
	}
	m.refresher = appsuggest.NewProfileIndexRefresher(loader, runtime, snapshot)

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

func (m *SuggestModule) ApplicationCapabilities() SuggestApplicationCapabilities {
	if m == nil {
		return SuggestApplicationCapabilities{}
	}
	return SuggestApplicationCapabilities{Service: m.service, RateLimit: m.config.RateLimit}
}

func (m *SuggestModule) RuntimeCapabilities() SuggestRuntimeCapabilities {
	if m == nil {
		return SuggestRuntimeCapabilities{}
	}
	return SuggestRuntimeCapabilities{Cleanup: m.Cleanup}
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
