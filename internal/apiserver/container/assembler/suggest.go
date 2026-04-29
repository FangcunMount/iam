package assembler

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/log"
	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	mysqlsuggest "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/suggest"
	searchruntime "github.com/FangcunMount/iam/internal/apiserver/infra/suggest/search"
)

// SuggestModule 联想搜索模块
type SuggestModule struct {
	Service *appsuggest.Service
	Updater *appsuggest.Updater

	config appsuggest.Config
	cancel context.CancelFunc
}

// NewSuggestModule 创建模块
func NewSuggestModule() *SuggestModule {
	return &SuggestModule{}
}

// Initialize 初始化模块
// params[0]: *gorm.DB
// params[1]: config.Config (可选，默认从 viper 读取)
func (m *SuggestModule) Initialize(params ...interface{}) error {
	var db *gorm.DB
	if len(params) > 0 {
		if v, ok := params[0].(*gorm.DB); ok {
			db = v
		}
	}

	cfg := appsuggest.LoadConfig()
	if len(params) > 1 {
		if v, ok := params[1].(appsuggest.Config); ok {
			cfg = v
		}
	}

	m.config = cfg

	if !cfg.Enable {
		log.Info("Suggest module disabled by config, skipping initialization")
		return nil
	}

	if db == nil {
		return fmt.Errorf("suggest module requires mysql connection")
	}

	runtime := searchruntime.NewRuntime()
	m.Service = appsuggest.NewServiceWithRuntime(appsuggest.Config{
		MaxResults: cfg.MaxResults,
		KeyPadLen:  cfg.KeyPadLen,
	}, runtime)

	loader := mysqlsuggest.NewLoader(db, mysqlsuggest.LoaderConfig{
		FullSQL:  cfg.FullSQL,
		DeltaSQL: cfg.DeltaSQL,
	})
	m.Updater = appsuggest.NewUpdaterWithRuntime(loader, cfg.ToUpdaterConfig(), runtime)

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Updater.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("start suggest updater: %w", err)
	}
	m.cancel = cancel

	log.Info("✅ Suggest module initialized")
	return nil
}

// Cleanup 停止调度
func (m *SuggestModule) Cleanup() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.Updater != nil {
		m.Updater.Stop()
	}
	return nil
}

// CheckHealth 检查是否已加载数据
func (m *SuggestModule) CheckHealth() error {
	if !m.config.Enable {
		return nil
	}
	if m.Service == nil {
		return fmt.Errorf("suggest service not initialized")
	}
	return nil
}

// ModuleInfo 返回模块信息
func (m *SuggestModule) ModuleInfo() ModuleInfo {
	return ModuleInfo{
		Name:        "suggest",
		Version:     "1.0.0",
		Description: "联想搜索模块",
	}
}
