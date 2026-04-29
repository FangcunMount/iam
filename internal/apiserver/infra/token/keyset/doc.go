// Package service 实现 JWKS 领域服务层
//
// 领域服务（Domain Service）负责实现复杂的业务逻辑，这些逻辑不适合放在实体或值对象中。
// 服务层实现 driving port 接口，被应用层调用，同时依赖 driven port 接口访问基础设施层。
//
// # 服务分类
//
// 1. KeyManager - 密钥生命周期管理服务
//   - 实现：jwks.Manager 接口
//   - 功能：创建密钥、获取密钥、状态转换、清理过期密钥
//   - 依赖：jwks.Repository, jwks.KeyGenerator
//
// 2. KeySetBuilder - JWKS 构建服务
//   - 实现：jwks.Publisher 接口
//   - 功能：构建 JWKS JSON、缓存管理、ETag 生成
//   - 依赖：jwks.Repository
//
// 3. KeyRotator - 密钥轮换服务（待实现）
//   - 实现：jwks.Rotator 接口
//   - 功能：自动轮换、策略管理
//   - 依赖：jwks.Repository, jwks.KeyGenerator
//
// # KeyManager 服务详情
//
// ## 核心方法
//
// - CreateKey: 创建新密钥
//   - 生成 UUID 作为 kid
//   - 调用 KeyGenerator 生成密钥对
//   - 创建 Key 实体并验证
//   - 保存到仓储
//
// - GetActiveKey: 获取当前激活密钥
//   - 查询 Active 状态的密钥
//   - 过滤出可签名且未过期的密钥
//   - 返回第一个符合条件的密钥
//
// - GetKeyByKid: 根据 kid 查询密钥
//   - 用于 Token 验证时查找公钥
//
// - RetireKey: 退役密钥（Grace → Retired）
//   - 只能对 Grace 状态的密钥执行
//   - 调用实体的 Retire() 方法进行状态转换
//
// - ForceRetireKey: 强制退役（任何状态 → Retired）
//   - 用于紧急情况（密钥泄露等）
//   - 调用实体的 ForceRetire() 方法
//
// - EnterGracePeriod: 进入宽限期（Active → Grace）
//   - 轮换密钥时将旧密钥转为 Grace 状态
//   - 调用实体的 EnterGrace() 方法
//
// - CleanupExpiredKeys: 清理过期密钥
//   - 查询所有过期密钥
//   - 删除 Retired 状态的过期密钥
//   - 强制退役其他过期密钥
//
// - ListKeys: 分页查询密钥
//   - 支持按状态过滤
//   - 支持分页参数
//
// ## 辅助方法
//
// - GetKeyStats: 获取密钥统计信息
//   - 统计各状态密钥数量
//   - 用于监控和展示
//
// - ValidateKeyHealth: 验证密钥健康状态
//   - 检查是否有可用的 Active 密钥
//   - 检查密钥是否即将过期（24小时）
//   - 用于健康检查端点
//
// # 错误处理
//
// 所有方法都使用 errors.WithCode() 包装错误，提供结构化的错误信息：
// - code.ErrDatabase: 数据库操作失败
// - code.ErrKeyNotFound: 密钥不存在
// - code.ErrNoActiveKey: 无可用的激活密钥
// - code.ErrInvalidStateTransition: 状态转换失败（由实体方法返回）
//
// # 事务处理
//
// 当前实现不包含事务管理，事务由应用层（Application Service）负责：
// - 单个密钥操作：通常不需要事务
// - 多个密钥操作（如轮换）：由 KeyRotator 或应用层管理事务
//
// # 示例用法
//
//	// 创建服务
//	keyManager := service.NewKeyManager(keyRepo, keyGenerator)
//
//	// 创建密钥
//	key, err := keyManager.CreateKey(ctx, "RS256", nil, nil)
//
//	// 获取当前激活密钥
//	activeKey, err := keyManager.GetActiveKey(ctx)
//
//	// 进入宽限期
//	err := keyManager.EnterGracePeriod(ctx, "old-key-id")
//
//	// 退役密钥
//	err := keyManager.RetireKey(ctx, "grace-key-id")
//
//	// 清理过期密钥
//	count, err := keyManager.CleanupExpiredKeys(ctx)
//
// # KeySetBuilder 服务详情
//
// ## 核心方法
//
// - BuildJWKS: 构建 JWKS JSON
//   - 查询所有可发布的密钥（Active + Grace 状态）
//   - 过滤出 ShouldPublish() 为 true 的密钥
//   - 按 kid 排序确保输出稳定
//   - 生成 ETag 和 Last-Modified
//   - 缓存构建结果（1分钟）
//
// - GetPublishableKeys: 获取可发布的密钥列表
//   - 用于预览或调试
//
// - ValidateCacheTag: 验证缓存标签
//   - 比较 ETag 或 Last-Modified
//   - 用于 HTTP 304 Not Modified 响应
//
// - GetCurrentCacheTag: 获取当前缓存标签
//   - 优先返回缓存的标签（1分钟内）
//   - 否则重新构建获取最新标签
//
// - RefreshCache: 刷新缓存
//   - 强制重新构建 JWKS
//   - 用于密钥轮换后立即更新
//
// ## 辅助方法
//
// - GetJWKSStats: 获取 JWKS 统计信息
//   - 统计可发布密钥数量（Active/Grace）
//   - 返回最后构建时间
//
// - GetCacheControl: 获取缓存控制策略
//   - 返回 HTTP Cache-Control 头值
//   - 推荐：public, max-age=3600, must-revalidate
//
// - ValidateJWKS: 验证 JWKS 完整性
//   - 检查是否有可发布的密钥
//   - 验证每个密钥的有效性
//
// ## 缓存机制
//
// KeySetBuilder 内置简单的缓存机制：
// - 缓存最后构建的 JWKS、CacheTag、构建时间
// - 缓存有效期：1分钟
// - 缓存自动刷新：GetCurrentCacheTag 时检查
//
// ## ETag 生成
//
// - 使用 SHA-256 哈希 JWKS JSON 内容
// - 取前 16 字节转十六进制
// - 格式：`"abc123..."`（带引号）
//
// ## 示例用法
//
//	// 创建服务
//	builder := service.NewKeySetBuilder(keyRepo)
//
//	// 构建 JWKS
//	jwksJSON, tag, err := builder.BuildJWKS(ctx)
//
//	// 验证客户端缓存
//	valid, err := builder.ValidateCacheTag(ctx, clientTag)
//	if valid {
//	    // 返回 304 Not Modified
//	}
//
//	// 刷新缓存（密钥轮换后）
//	err := builder.RefreshCache(ctx)
package jwks
