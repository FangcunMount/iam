package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	messagingInfra "github.com/FangcunMount/iam/internal/apiserver/infra/messaging"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	eventoutbox "github.com/FangcunMount/iam/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/FangcunMount/iam/internal/pkg/eventruntime"
)

// Container 容器
// 负责管理所有模块的依赖注入和生命周期
type Container struct {
	// 数据库连接
	mysqlDB     *gorm.DB
	redisClient *redis.Client // Redis（缓存、令牌等）

	// 消息总线（可选）
	eventBus messaging.EventBus

	// 事件平台
	eventCatalog   *eventcatalog.Catalog
	eventPublisher event.Publisher
	outboxStore    *eventoutbox.Store
	outboxRelay    messagingInfra.OutboxRelay

	// 业务模块
	AuthnModule            *assembler.AuthnModule
	UserModule             *assembler.UserModule
	AuthzModule            *assembler.AuthzModule
	IDPModule              *assembler.IDPModule
	SuggestModule          *assembler.SuggestModule
	CacheGovernanceService *cachegovernance.ReadService

	// IDP 模块加密密钥（32 字节 AES-256）
	idpEncryptionKey []byte

	// 容器状态
	initialized bool

	// typed runtime options
	runtimeOptions RuntimeOptions
}

// NewContainer 创建容器
// redisClient: Redis 客户端（用于缓存、令牌等）
// eventBus: 消息总线（可选，用于事件驱动，传 nil 则不使用消息队列）
// encryptionKey: IDP 模块使用的加密密钥（32 字节 AES-256），传 nil 则使用默认密钥
func NewContainer(mysqlDB *gorm.DB, redisClient *redis.Client, eventBus messaging.EventBus, encryptionKey []byte) *Container {
	return NewContainerWithOptions(mysqlDB, redisClient, eventBus, encryptionKey, RuntimeOptionsFromAPIServerOptions(nil, ""))
}

// NewContainerWithOptions 创建带 typed runtime options 的容器。
func NewContainerWithOptions(mysqlDB *gorm.DB, redisClient *redis.Client, eventBus messaging.EventBus, encryptionKey []byte, opts RuntimeOptions) *Container {
	return &Container{
		mysqlDB:          mysqlDB,
		redisClient:      redisClient,
		eventBus:         eventBus,
		idpEncryptionKey: encryptionKey,
		runtimeOptions:   opts,
	}
}

// Initialize 初始化容器
func (c *Container) Initialize() error {
	if c.initialized {
		return fmt.Errorf("container already initialized")
	}

	var errors []error

	// 1. 初始化事件平台（catalog + outbox store + relay）
	if err := c.initEventing(); err != nil {
		log.Warnf("Failed to initialize event runtime: %v", err)
		errors = append(errors, fmt.Errorf("event runtime: %w", err))
	}

	// 2. 初始化 IDP 模块（先初始化，因为 authn 模块依赖它）
	if err := c.initIDPModule(); err != nil {
		log.Warnf("Failed to initialize IDP module: %v", err)
		errors = append(errors, fmt.Errorf("idp module: %w", err))
	}

	// 3. 初始化认证模块（依赖 IDP 模块）
	if err := c.initAuthModule(); err != nil {
		log.Warnf("Failed to initialize Authn module: %v", err)
		errors = append(errors, fmt.Errorf("authn module: %w", err))
	}

	// 4. 初始化授权模块（用户模块 /identity/me 的 roles 依赖 Casbin）
	if err := c.initAuthzModule(); err != nil {
		log.Warnf("Failed to initialize Authz module: %v", err)
		errors = append(errors, fmt.Errorf("authz module: %w", err))
	}

	// 5. 初始化用户模块
	if err := c.initUserModule(); err != nil {
		log.Warnf("Failed to initialize User module: %v", err)
		errors = append(errors, fmt.Errorf("user module: %w", err))
	}

	// 6. 初始化 Suggest 模块（可选）
	if err := c.initSuggestModule(); err != nil {
		log.Warnf("Failed to initialize Suggest module: %v", err)
		errors = append(errors, fmt.Errorf("suggest module: %w", err))
	}

	// 7. 初始化只读缓存治理服务
	c.initCacheGovernance()

	c.initialized = true

	// 打印初始化状态
	log.Infof("🏗️  Container initialization completed:")
	if c.IDPModule != nil {
		log.Info("   ✅ IDP module")
	} else {
		log.Warn("   ❌ IDP module failed")
	}
	if c.AuthnModule != nil {
		log.Info("   ✅ Authn module")
	} else {
		log.Warn("   ❌ Authn module failed")
	}
	if c.UserModule != nil {
		log.Info("   ✅ User module")
	} else {
		log.Warn("   ❌ User module failed")
	}
	if c.AuthzModule != nil {
		log.Info("   ✅ Authz module")
	} else {
		log.Warn("   ❌ Authz module failed")
	}
	if c.SuggestModule != nil && c.SuggestModule.Service != nil {
		log.Info("   ✅ Suggest module")
	} else {
		log.Warn("   ⚠️  Suggest module not initialized or disabled")
	}
	if c.outboxStore != nil {
		log.Info("   ✅ Event outbox")
	} else {
		log.Warn("   ⚠️  Event outbox not initialized")
	}

	// 如果有错误,返回组合错误(但容器仍然标记为已初始化)
	if len(errors) > 0 {
		return fmt.Errorf("some modules failed to initialize (%d errors)", len(errors))
	}

	return nil
}

func (c *Container) initEventing() error {
	catalogPath := strings.TrimSpace(c.runtimeOptions.Events.CatalogPath)
	if catalogPath == "" {
		catalogPath = "configs/events.yaml"
	}
	cfg, err := eventcatalog.Load(catalogPath)
	if err != nil {
		return fmt.Errorf("load event catalog %q: %w", catalogPath, err)
	}
	catalog := eventcatalog.NewCatalog(cfg)
	c.eventCatalog = catalog
	c.eventPublisher = eventruntime.NewPublisherForBus(catalog, c.eventBus)
	if c.mysqlDB == nil {
		return nil
	}
	c.outboxStore = eventoutbox.NewStore(c.mysqlDB, catalog)
	if c.eventBus == nil {
		log.Warnw("event outbox relay not started: event bus unavailable", "store", "iam.domain_event_outbox")
		return nil
	}
	c.outboxRelay = messagingInfra.NewOutboxRelay("iam.domain_event_outbox", c.outboxStore, c.eventBus, c.outboxRelayOptions())
	return nil
}

func (c *Container) outboxRelayOptions() messagingInfra.OutboxRelayOptions {
	return messagingInfra.OutboxRelayOptions{
		BatchSize:  c.runtimeOptions.Events.OutboxRelayBatchSize,
		RetryDelay: c.runtimeOptions.Events.OutboxRelayRetryDelay,
	}
}

// initAuthModule 初始化认证模块（依赖 IDP 模块）
// 认证模块使用 Redis 进行 Token 持久化存储
func (c *Container) initAuthModule() error {
	authModule := assembler.NewAuthnModule()
	if err := authModule.InitializeWithDeps(assembler.AuthnModuleDeps{
		DB:             c.mysqlDB,
		RedisClient:    c.redisClient,
		IDPModule:      c.IDPModule,
		EventBus:       c.eventBus,
		EventPublisher: c.eventPublisher,
		AppMode:        c.runtimeOptions.AppMode,
		Auth:           c.runtimeOptions.Auth,
		JWKS:           c.runtimeOptions.JWKS,
		SMS:            c.runtimeOptions.SMS,
	}); err != nil {
		return fmt.Errorf("failed to initialize auth module: %w", err)
	}
	c.AuthnModule = authModule
	return nil
}

// initUserModule 初始化用户模块
func (c *Container) initUserModule() error {
	userModule := assembler.NewUserModule()
	if err := userModule.InitializeWithDeps(c.moduleGraph().userModuleDependencies()); err != nil {
		return fmt.Errorf("failed to initialize user module: %w", err)
	}
	c.UserModule = userModule
	return nil
}

// initAuthzModule 初始化授权模块
// 授权模块通过 outbox stager 记录 durable 策略版本事件
func (c *Container) initAuthzModule() error {
	authzModule := assembler.NewAuthzModule()

	if err := authzModule.InitializeWithDeps(assembler.AuthzModuleDeps{
		DB:          c.mysqlDB,
		EventStager: c.outboxStore,
	}); err != nil {
		return fmt.Errorf("failed to initialize authz module: %w", err)
	}
	c.AuthzModule = authzModule
	return nil
}

// initSuggestModule 初始化联想模块
func (c *Container) initSuggestModule() error {
	suggestModule := assembler.NewSuggestModule()
	if err := suggestModule.InitializeWithDeps(assembler.SuggestModuleDeps{
		DB:     c.mysqlDB,
		Config: c.runtimeOptions.Suggest,
	}); err != nil {
		return fmt.Errorf("failed to initialize suggest module: %w", err)
	}
	// 可能因配置关闭而 Service 为空
	if suggestModule.Service != nil {
		c.SuggestModule = suggestModule
	}
	return nil
}

// initIDPModule 初始化 IDP 模块（Identity Provider）
// IDP 模块使用 Redis 缓存 Access Token
func (c *Container) initIDPModule() error {
	idpModule := assembler.NewIDPModule()
	// 传递 Redis（用于 Access Token 缓存）
	if err := idpModule.InitializeWithDeps(assembler.IDPModuleDeps{
		DB:            c.mysqlDB,
		RedisClient:   c.redisClient,
		EncryptionKey: c.idpEncryptionKey,
	}); err != nil {
		return fmt.Errorf("failed to initialize idp module: %w", err)
	}
	c.IDPModule = idpModule
	return nil
}

type cacheInspectorProvider interface {
	CacheFamilyInspectors() []cachegovernance.FamilyInspector
}

func (c *Container) initCacheGovernance() {
	inspectors := make([]cachegovernance.FamilyInspector, 0, 8)
	for _, provider := range []cacheInspectorProvider{c.AuthnModule, c.IDPModule} {
		if provider == nil {
			continue
		}
		inspectors = append(inspectors, provider.CacheFamilyInspectors()...)
	}
	c.CacheGovernanceService = cachegovernance.NewReadService(inspectors)
}

// HealthCheck 健康检查
func (c *Container) HealthCheck(ctx context.Context) error {
	// 检查MySQL连接
	if c.mysqlDB != nil {
		if err := c.mysqlDB.WithContext(ctx).Raw("SELECT 1").Error; err != nil {
			return fmt.Errorf("mysql health check failed: %w", err)
		}
	}

	// 检查 Redis 连接
	if c.redisClient != nil {
		if err := c.redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis health check failed: %w", err)
		}
	}

	return nil
}

// GetMySQLDB 获取MySQL数据库连接
func (c *Container) GetMySQLDB() *gorm.DB {
	return c.mysqlDB
}

func (c *Container) OutboxRelay() messagingInfra.OutboxRelay {
	if c == nil {
		return nil
	}
	return c.outboxRelay
}

// IsInitialized 检查容器是否已初始化
func (c *Container) IsInitialized() bool {
	return c.initialized
}

// PrintStatus 打印容器状态
func (c *Container) PrintStatus() {
	fmt.Printf("📊 Container Status:\n")
	fmt.Printf("   • Initialized: %t\n", c.initialized)

	// 数据库连接状态
	fmt.Printf("   • MySQL: ")
	if c.mysqlDB != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	fmt.Printf("   • Redis: ")
	if c.redisClient != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	// 模块状态
	fmt.Printf("   • Authn Module: ")
	if c.AuthnModule != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	fmt.Printf("   • User Module: ")
	if c.UserModule != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	fmt.Printf("   • Authz Module: ")
	if c.AuthzModule != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	fmt.Printf("   • IDP Module: ")
	if c.IDPModule != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("❌\n")
	}

	fmt.Printf("   • Suggest Module: ")
	if c.SuggestModule != nil && c.SuggestModule.Service != nil {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("⚠️  (disabled or not initialized)\n")
	}
}
