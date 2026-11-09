package migration

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
)

// migrations 嵌入迁移文件
// 这样打包后的二进制文件中就包含了迁移 SQL，无需挂载外部文件
//
//go:embed migrations/*.sql
var migrations embed.FS

// Config 迁移配置
type FreshStage struct {
	Version uint
	Prepare func() error
}

type Config struct {
	// FreshStages run only after proving that no application table existed before
	// this migration invocation. Existing installations never execute these hooks.
	FreshStages []FreshStage

	Enabled  bool   // 是否启用自动迁移
	Database string // 数据库名称
}

// Migrator 数据库迁移器
type Migrator struct {
	db     *sql.DB
	config *Config
}

// NewMigrator 创建迁移器
func NewMigrator(db *sql.DB, config *Config) *Migrator {
	return &Migrator{
		db:     db,
		config: config,
	}
}

// Run 执行数据库迁移并返回最新版本以及是否执行了迁移
//
// 工作流程:
// 1. 检查是否启用迁移
// 2. 创建 migrate 实例
// 3. 获取当前版本
// 4. 执行迁移到最新版本
// 5. 返回最新版本及是否执行了迁移
func (m *Migrator) Run() (uint, bool, error) {
	if !m.config.Enabled {
		return 0, false, nil
	}

	// 创建 migrate 实例
	ctx := context.Background()
	session, err := m.lockSession(ctx)
	if err != nil {
		return 0, false, err
	}
	defer session.close()
	instance, err := session.openWithoutPendingBootstrap(ctx)
	if err != nil {
		return 0, false, err
	}

	// 获取当前版本
	currentVersion, dirty, err := instance.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return 0, false, fmt.Errorf("failed to get current version: %w", err)
	}

	var versionBefore uint
	if err == migrate.ErrNilVersion {
		versionBefore = 0
	} else {
		versionBefore = currentVersion
	}

	if dirty {
		return versionBefore, false, fmt.Errorf("database is in dirty state at version %d, please fix manually", versionBefore)
	}

	if versionBefore == 0 && len(m.config.FreshStages) > 0 {
		var tables int
		if err := session.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME <> 'schema_migrations'").Scan(&tables); err != nil {
			return 0, false, err
		}
		if tables != 0 {
			return 0, false, fmt.Errorf("fresh bootstrap requires an empty database")
		}
		for _, stage := range m.config.FreshStages {
			if err := instance.Migrate(stage.Version); err != nil && err != migrate.ErrNoChange {
				return 0, false, err
			}
			if stage.Prepare == nil {
				return stage.Version, true, fmt.Errorf("fresh migration preparation missing")
			}
			if err := stage.Prepare(); err != nil {
				return stage.Version, true, fmt.Errorf("fresh migration preparation at %d: %w", stage.Version, err)
			}
		}
	}

	// 执行迁移
	if err := instance.Up(); err != nil {
		if err == migrate.ErrNoChange {
			// Fresh preparation may already have advanced to the latest schema.
			current, _, versionErr := instance.Version()
			if versionErr != nil {
				return versionBefore, false, versionErr
			}
			return current, current != versionBefore, nil
		}
		return versionBefore, false, fmt.Errorf("migration failed: %w", err)
	}

	// 获取新版本
	newVersion, _, verr := instance.Version()
	if verr != nil {
		return versionBefore, true, fmt.Errorf("failed to get new version: %w", verr)
	}

	return newVersion, true, nil
}

// RunTo migrates to an explicit schema version. It is used by the maintenance
// window to stop at the additive AuthZ schema before the offline conversion.
func (m *Migrator) RunTo(target uint) (uint, bool, error) {
	if !m.config.Enabled {
		return 0, false, nil
	}
	ctx := context.Background()
	session, err := m.lockSession(ctx)
	if err != nil {
		return 0, false, err
	}
	defer session.close()
	instance, err := session.openWithoutPendingBootstrap(ctx)
	if err != nil {
		return 0, false, err
	}

	current, dirty, versionErr := instance.Version()
	if versionErr != nil && versionErr != migrate.ErrNilVersion {
		return 0, false, fmt.Errorf("failed to get current version: %w", versionErr)
	}
	if versionErr == migrate.ErrNilVersion {
		current = 0
	}
	if dirty {
		return current, false, fmt.Errorf("database is in dirty state at version %d, please fix manually", current)
	}
	if current > target {
		return current, false, fmt.Errorf("refusing to migrate database backwards from version %d to %d", current, target)
	}
	if current == target {
		return current, false, nil
	}
	if err := instance.Migrate(target); err != nil {
		if err == migrate.ErrNoChange {
			return current, false, nil
		}
		return current, false, fmt.Errorf("migration to version %d failed: %w", target, err)
	}
	newVersion, _, err := instance.Version()
	if err != nil {
		return current, true, fmt.Errorf("failed to get new version: %w", err)
	}
	return newVersion, true, nil
}
