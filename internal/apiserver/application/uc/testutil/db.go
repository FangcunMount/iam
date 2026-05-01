package testutil

import (
	"fmt"
	"os"
	"testing"

	ucuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/uow"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	profilepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/profile"
	profilelinkpo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/profilelink"
	mysqluow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/uc"
	userpo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
)

// SetupTestDB 创建内存数据库用于测试
// 使用 SQLite 内存模式，快速且无需外部依赖
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// 创建每个测试使用的临时 sqlite 文件数据库，启用 WAL 和 busy_timeout
	// 这样可以在并发测试中减少锁争用噪声（比共享内存模式更稳定）
	tmp, err := os.CreateTemp("", "testdb-*.db")
	require.NoError(t, err, "failed to create temp db file")
	tmpPath := tmp.Name()
	// 关闭并让 GORM 打开自己的连接
	_ = tmp.Close()

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=10000", tmpPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent), // 测试时静默日志
		DisableForeignKeyConstraintWhenMigrating: true,                                  // SQLite 兼容性
	})
	require.NoError(t, err, "failed to create sqlite temp database")

	// 限制底层 sql.DB 连接池，避免并发连接导致 sqlite 锁竞争
	if sqlDB, cerr := db.DB(); cerr == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}

	// 在测试结束时清理：关闭连接并删除临时文件
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		_ = os.Remove(tmpPath)
	})

	// 自动迁移所有表
	err = db.AutoMigrate(
		&userpo.UserPO{},
		&profilepo.ProfilePO{},
		&profilelinkpo.ProfileLinkPO{},
	)
	require.NoError(t, err, "failed to auto-migrate tables")

	return db
}

// NewUnitOfWork returns the UC UnitOfWork backed by the test database.
func NewUnitOfWork(db *gorm.DB) ucuow.UnitOfWork {
	return mysqluow.NewUnitOfWork(db)
}

// CleanupDB 清空数据库所有表（用于每个测试之间清理）
func CleanupDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	// 按依赖顺序删除（先删除有外键的表）
	tables := []string{
		"profile_links", // 档案关系（依赖 users 和 profiles）
		"profiles",      // 档案
		"users",         // 用户
	}

	for _, table := range tables {
		err := db.Exec("DELETE FROM " + table).Error
		require.NoError(t, err, "failed to cleanup table: %s", table)
	}
}
