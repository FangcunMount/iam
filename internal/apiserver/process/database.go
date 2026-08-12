package process

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/database"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v3/internal/apiserver/config"
	"github.com/FangcunMount/iam/v3/internal/pkg/migration"
	"github.com/FangcunMount/iam/v3/internal/pkg/options"
)

// DatabaseManager 数据库管理器
type DatabaseManager struct {
	config        *config.Config
	registry      *database.Registry
	cacheRegistry *database.NamedRedisRegistry
}

// NewDatabaseManager 创建数据库管理器
func NewDatabaseManager(config *config.Config) *DatabaseManager {
	return &DatabaseManager{
		config:   config,
		registry: database.NewRegistry(),
	}
}

// Initialize 初始化数据库连接
func (dm *DatabaseManager) Initialize() error {
	log.Info("🔌 Initializing database connections...")

	// 初始化MySQL连接
	if err := dm.initMySQL(); err != nil {
		log.Warnf("Failed to initialize MySQL: %v", err)
		// 不返回错误，允许应用在没有MySQL的情况下运行
	}

	// 初始化 Redis 客户端
	if err := dm.initRedisClients(); err != nil {
		log.Warnf("Failed to initialize Redis clients: %v", err)
		// 不返回错误，允许应用在没有Redis的情况下运行
	}

	// 初始化数据库连接
	if err := dm.registry.Init(); err != nil {
		log.Warnf("Failed to initialize database connections: %v", err)
		// 不返回错误，允许应用在没有数据库的情况下运行
	}

	// 执行数据库迁移
	if err := dm.runMigrations(); err != nil {
		log.Errorf("Failed to run database migrations: %v", err)
		return err // 迁移失败应该终止启动
	}

	log.Info("Database connections initialization completed")
	return nil
}

// runMigrations 执行数据库迁移
func (dm *DatabaseManager) runMigrations() error {
	// 检查是否启用迁移
	if !dm.config.MigrationOptions.Enabled {
		log.Info("📦 Database migration is disabled")
		return nil
	}

	log.Info("🔄 Starting database migration...")

	// 获取 MySQL 连接
	gormDB, err := dm.GetMySQLDB()
	if err != nil {
		log.Warnf("Cannot run migration: MySQL not available: %v", err)
		return nil // 如果没有 MySQL，跳过迁移
	}

	// 确保 gormDB 不为 nil
	if gormDB == nil {
		log.Warn("Cannot run migration: MySQL connection is nil")
		return nil
	}

	// 确保数据库存在（在执行迁移前）
	if err := dm.ensureDatabase(gormDB); err != nil {
		return fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// 创建独立的 *sql.DB 供迁移使用（防止关闭迁移连接影响业务连接）
	dsn := fmt.Sprintf(`%s:%s@tcp(%s)/%s?charset=utf8&parseTime=%t&loc=%s&multiStatements=true`,
		dm.config.MySQLOptions.Username,
		dm.config.MySQLOptions.Password,
		dm.config.MySQLOptions.Host,
		dm.config.MySQLOptions.Database,
		true,
		"Local",
	)
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open migration database connection: %w", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Debugf("Failed to close migration database connection: %v", cerr)
		}
	}()
	sqlDB.SetMaxOpenConns(dm.config.MySQLOptions.MaxOpenConnections)
	sqlDB.SetConnMaxLifetime(dm.config.MySQLOptions.MaxConnectionLifeTime)
	sqlDB.SetMaxIdleConns(dm.config.MySQLOptions.MaxIdleConnections)
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping migration database: %w", err)
	}

	// 创建迁移器
	migrator := migration.NewMigrator(sqlDB, &migration.Config{
		Enabled:  dm.config.MigrationOptions.Enabled,
		Database: dm.config.MigrationOptions.Database,
	})

	// 执行迁移
	version, applied, err := migrator.Run()
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	switch {
	case applied:
		log.Infof("✅ Database migration completed successfully (version: %d)", version)
	case version == 0:
		log.Infof("✅ No database migrations applied (version: %d)", version)
	default:
		log.Infof("✅ Database already up to date (version: %d)", version)
	}

	return nil
}

// ensureDatabase 确保数据库存在
func (dm *DatabaseManager) ensureDatabase(gormDB *gorm.DB) error {
	dbName := dm.config.MigrationOptions.Database

	// 检查数据库是否存在
	var exists int64
	err := gormDB.Raw("SELECT COUNT(*) FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&exists).Error
	if err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	// 如果数据库不存在，创建它
	if exists == 0 {
		log.Infof("Database '%s' does not exist, creating...", dbName)
		createSQL := fmt.Sprintf(
			"CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_unicode_ci",
			dbName,
		)
		if err := gormDB.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		log.Infof("✅ Database '%s' created successfully", dbName)
	} else {
		log.Debugf("Database '%s' already exists", dbName)
	}

	return nil
}

// initMySQL 初始化MySQL连接
func (dm *DatabaseManager) initMySQL() error {
	// 创建 GORM logger 实例
	gormLogger := logger.NewGormLogger(dm.config.MySQLOptions.LogLevel)

	// 打印日志配置信息，便于调试
	log.Infof("Initializing MySQL with log level: %d", dm.config.MySQLOptions.LogLevel)

	mysqlConfig := &database.MySQLConfig{
		Host:                  dm.config.MySQLOptions.Host,
		Username:              dm.config.MySQLOptions.Username,
		Password:              dm.config.MySQLOptions.Password,
		Database:              dm.config.MySQLOptions.Database,
		MaxIdleConnections:    dm.config.MySQLOptions.MaxIdleConnections,
		MaxOpenConnections:    dm.config.MySQLOptions.MaxOpenConnections,
		MaxConnectionLifeTime: dm.config.MySQLOptions.MaxConnectionLifeTime,
		LogLevel:              dm.config.MySQLOptions.LogLevel,
		Logger:                gormLogger,
	}

	if mysqlConfig.Host == "" {
		log.Info("MySQL host not configured, skipping MySQL initialization")
		return nil
	}

	mysqlConn := database.NewMySQLConnection(mysqlConfig)
	return dm.registry.Register(database.MySQL, mysqlConfig, mysqlConn)
}

// initRedisClients 初始化 Redis 客户端
func (dm *DatabaseManager) initRedisClients() error {
	// 初始化 Cache Redis
	if err := dm.initSingleRedis("cache", dm.config.RedisOptions.Cache); err != nil {
		log.Warnf("Failed to initialize Cache Redis: %v", err)
	}

	return nil
}

// initSingleRedis 初始化单个 Redis 客户端
func (dm *DatabaseManager) initSingleRedis(instanceName string, opts *options.SingleRedisOptions) error {
	if opts == nil {
		return fmt.Errorf("%s redis options is nil", instanceName)
	}

	if opts.Host == "" {
		log.Infof("Redis %s host not configured, skipping initialization", instanceName)
		return nil
	}

	log.Infof("Initializing Redis %s connection to %s:%d (username: %q, password: %s, db: %d, log_enabled: %v)",
		instanceName,
		opts.Host,
		opts.Port,
		opts.Username,
		func() string {
			if opts.Password == "" {
				return "<empty>"
			}
			return "<set>"
		}(),
		opts.Database,
		opts.EnableLogging,
	)

	// 创建 Redis 配置
	redisConfig := &database.RedisConfig{
		Host:                  opts.Host,
		Port:                  opts.Port,
		Addrs:                 opts.Addrs,
		Username:              opts.Username,
		Password:              opts.Password,
		Database:              opts.Database,
		MaxIdle:               opts.MaxIdle,
		MaxActive:             opts.MaxActive,
		Timeout:               opts.Timeout,
		MinIdleConns:          opts.MinIdleConns,
		PoolTimeout:           opts.PoolTimeout,
		DialTimeout:           opts.DialTimeout,
		ReadTimeout:           opts.ReadTimeout,
		WriteTimeout:          opts.WriteTimeout,
		EnableCluster:         opts.EnableCluster,
		UseSSL:                opts.UseSSL,
		SSLInsecureSkipVerify: opts.SSLInsecureSkipVerify,
	}

	// 通过 Foundation runtime 管理 Redis 生命周期。
	registry := database.NewNamedRedisRegistry(redisConfig, nil)
	if err := registry.Connect(); err != nil {
		return fmt.Errorf("failed to connect Redis %s: %w", instanceName, err)
	}
	dm.cacheRegistry = registry

	// 获取连接的客户端并添加日志钩子
	if opts.EnableLogging {
		client, err := dm.getRedisClientFromCacheRegistry()
		if err != nil {
			log.Warnf("Failed to get Redis %s client for logging hook: %v", instanceName, err)
		} else {
			log.Infof("Enabling Redis %s command logging (slow threshold: 200ms)", instanceName)
			redisHook := logger.NewRedisHook(true, 200*time.Millisecond)
			client.AddHook(redisHook)
		}
	}

	log.Infof("✅ Redis %s connected successfully: %s:%d (db: %d)", instanceName, opts.Host, opts.Port, opts.Database)
	return nil
}

// getRedisClientFromCacheRegistry 从 Foundation runtime 获取缓存 Redis 客户端。
func (dm *DatabaseManager) getRedisClientFromCacheRegistry() (*redis.Client, error) {
	if dm.cacheRegistry == nil {
		return nil, fmt.Errorf("cache redis registry is not initialized")
	}

	client, err := dm.cacheRegistry.GetClient("")
	if err != nil {
		return nil, err
	}

	// component-base 返回的是 redis.UniversalClient，需要转换为 *redis.Client
	switch c := client.(type) {
	case *redis.Client:
		return c, nil
	case redis.UniversalClient:
		// 尝试转换为 *redis.Client
		if redisClient, ok := c.(*redis.Client); ok {
			return redisClient, nil
		}
		return nil, fmt.Errorf("redis client is UniversalClient but not *redis.Client")
	default:
		return nil, fmt.Errorf("invalid redis client type: %T", client)
	}
}

// GetMySQLDB 获取MySQL数据库连接
func (dm *DatabaseManager) GetMySQLDB() (*gorm.DB, error) {
	client, err := dm.registry.GetClient(database.MySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to get MySQL client: %w", err)
	}

	mysqlClient, ok := client.(*gorm.DB)
	if !ok {
		return nil, fmt.Errorf("invalid MySQL client type")
	}

	return mysqlClient, nil
}

// GetCacheRedisClient 获取缓存 Redis 客户端
// 用于缓存、会话、限流等临时数据
func (dm *DatabaseManager) GetCacheRedisClient() (*redis.Client, error) {
	return dm.getRedisClientFromCacheRegistry()
}

// Close 关闭所有数据库连接
func (dm *DatabaseManager) Close() error {
	var lastErr error
	if dm.cacheRegistry != nil {
		if err := dm.cacheRegistry.Close(); err != nil {
			lastErr = err
		}
	}
	if err := dm.registry.Close(); err != nil {
		lastErr = err
	}
	return lastErr
}

// HealthCheck 健康检查
func (dm *DatabaseManager) HealthCheck(ctx context.Context) error {
	if err := dm.registry.HealthCheck(ctx); err != nil {
		return err
	}
	if dm.cacheRegistry != nil {
		if err := dm.cacheRegistry.HealthCheck(ctx); err != nil {
			return err
		}
	}
	return nil
}
