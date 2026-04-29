package suggest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/FangcunMount/component-base/pkg/log"
)

// Loader 负责提供数据行
type Loader interface {
	Full(ctx context.Context) ([]string, error)
	Delta(ctx context.Context, since time.Time) ([]string, error)
}

// Updater 定期刷新内存搜索引擎
type Updater struct {
	loader        Loader
	runtime       IndexRuntime
	fullSpec      string
	deltaSpec     string
	dataDir       string
	lastFetch     time.Time
	cron          *cron.Cron
	writeSnapshot bool
}

// UpdaterConfig 配置刷新策略
type UpdaterConfig struct {
	FullCron  string
	DeltaCron string
	DataDir   string
	Snapshot  bool
}

// NewUpdater 创建 Updater
func NewUpdater(loader Loader, cfg UpdaterConfig) *Updater {
	return NewUpdaterWithRuntime(loader, cfg, nil)
}

// NewUpdaterWithRuntime creates an updater with an explicit index runtime.
func NewUpdaterWithRuntime(loader Loader, cfg UpdaterConfig, runtime IndexRuntime) *Updater {
	full := cfg.FullCron
	if full == "" {
		full = "@every 1h"
	}
	return &Updater{
		loader:        loader,
		runtime:       runtime,
		fullSpec:      full,
		deltaSpec:     cfg.DeltaCron,
		dataDir:       cfg.DataDir,
		writeSnapshot: cfg.Snapshot,
	}
}

// Start 启动调度
func (u *Updater) Start(ctx context.Context) error {
	if u.loader == nil {
		return fmt.Errorf("suggest updater missing loader")
	}

	if err := u.runFull(ctx); err != nil {
		return err
	}

	u.cron = cron.New()

	if _, err := u.cron.AddFunc(u.fullSpec, func() {
		if err := u.runFull(ctx); err != nil {
			log.Errorw("suggest full sync failed", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("add full cron failed: %w", err)
	}

	if u.deltaSpec != "" {
		if _, err := u.cron.AddFunc(u.deltaSpec, func() {
			if err := u.runDelta(ctx); err != nil {
				log.Errorw("suggest delta sync failed", "error", err)
			}
		}); err != nil {
			return fmt.Errorf("add delta cron failed: %w", err)
		}
	}

	u.cron.Start()

	go func() {
		<-ctx.Done()
		u.Stop()
	}()

	return nil
}

// Stop 停止调度
func (u *Updater) Stop() {
	if u.cron == nil {
		return
	}
	ctx := u.cron.Stop()
	<-ctx.Done()
}

func (u *Updater) runFull(ctx context.Context) error {
	lines, err := u.loader.Full(ctx)
	if err != nil {
		return err
	}
	if u.runtime != nil {
		u.runtime.Replace(lines)
	}
	u.lastFetch = time.Now()
	u.persist(lines)
	log.Infow("suggest full sync completed", "count", len(lines))
	return nil
}

func (u *Updater) runDelta(ctx context.Context) error {
	if u.lastFetch.IsZero() {
		return nil
	}
	lines, err := u.loader.Delta(ctx, u.lastFetch)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	if u.runtime == nil {
		return fmt.Errorf("suggest store not initialized")
	}
	if err := u.runtime.ImportDelta(lines); err != nil {
		return err
	}
	u.lastFetch = time.Now()
	u.persist(lines)
	log.Infow("suggest delta sync completed", "count", len(lines))
	return nil
}

func (u *Updater) persist(lines []string) {
	if !u.writeSnapshot || len(lines) == 0 || u.dataDir == "" {
		return
	}
	if err := os.MkdirAll(u.dataDir, 0o755); err != nil {
		log.Warnw("suggest persist mkdir failed", "error", err, "dir", u.dataDir)
		return
	}
	file := filepath.Join(u.dataDir, "snapshot.txt")
	if err := os.WriteFile(file, []byte(joinLines(lines)), 0o644); err != nil {
		log.Warnw("suggest persist snapshot failed", "error", err, "file", file)
	}
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
