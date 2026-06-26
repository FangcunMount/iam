package container

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/messaging"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/suggest"
	messagingInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/messaging"
	eventoutbox "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/FangcunMount/iam/v2/pkg/eventcatalog"
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
	AuthnModule            *authn.AuthnModule
	IdentityModule         *identity.IdentityModule
	AuthzModule            *authz.AuthzModule
	IDPModule              *idp.IDPModule
	SuggestModule          *suggest.SuggestModule
	CacheGovernanceService *cachegovernance.ReadService

	// IDP 模块加密密钥（32 字节 AES-256）
	idpEncryptionKey []byte

	// 容器状态
	initialized     bool
	bootstrapErrors map[string]string

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

	c.bootstrapErrors = make(map[string]string)
	errors := c.runBootstrapPlan()
	c.initialized = true
	c.logBootstrapStatus()

	// 如果有错误,返回组合错误(但容器仍然标记为已初始化)
	if len(errors) > 0 {
		return fmt.Errorf("some modules failed to initialize (%d errors)", len(errors))
	}

	return nil
}

// HealthCheck 健康检查
func (c *Container) HealthCheck(ctx context.Context) error {
	if c.mysqlDB != nil {
		if err := c.mysqlDB.WithContext(ctx).Raw("SELECT 1").Error; err != nil {
			return fmt.Errorf("mysql health check failed: %w", err)
		}
	}

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

	fmt.Printf("   • Identity Module: ")
	if c.IdentityModule != nil {
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
	if c.SuggestModule != nil && c.SuggestModule.IsInitialized() {
		fmt.Printf("✅\n")
	} else {
		fmt.Printf("⚠️  (disabled or not initialized)\n")
	}
}
