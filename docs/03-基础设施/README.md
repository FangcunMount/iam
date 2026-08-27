# IAM 基础设施设计

> 状态：已实现 · 本目录以当前组合根、基础设施适配器、配置、迁移和测试为依据。

## 1. 本目录回答什么

业务模块说明 IAM **做什么**，本目录说明这些业务能力依靠什么技术机制才能在失败、并发和多实例条件下成立。

IAM 当前最重要的基础设施问题不是“用了哪些组件”，而是下面六个问题：

| 问题 | 当前选择 | canonical 文档 |
| --- | --- | --- |
| 跨多个仓储的写入如何保持原子性 | MySQL + GORM Unit of Work + 数据库约束 | [MySQL、事务与迁移](01-MySQL事务与迁移.md) |
| 短期认证状态如何保存并处理并发 | Redis String/ZSet + Lua/WATCH + TTL | [Redis 与缓存一致性](02-Redis与缓存一致性.md) |
| 数据库提交后如何可靠传播事件 | 事件目录 + Transactional Outbox + relay | [事件与 Transactional Outbox](03-事件与Transactional-Outbox.md) |
| 私钥、应用密钥和 JWT 密钥如何管理 | AES-GCM、Argon2id、PEM 私钥、MySQL 元数据、JWKS 公钥快照 | [密码学、密钥与令牌](04-密码学密钥与令牌.md) |
| REST/gRPC 的传输和服务身份如何保护 | TLS/mTLS、拦截器链、JWT middleware、ACL | [传输层与服务间安全](05-传输层与服务间安全.md) |
| 进程如何判断可接流量并安全退出 | liveness/readiness、低基数指标、draining、分阶段关闭 | [可观测性、就绪与关闭](06-可观测性就绪与关闭.md) |

## 2. 30 秒结论

IAM 的基础设施设计可以归纳为四条原则：

1. **MySQL 保存需要跨重启恢复的业务事实**。领域对象、授权事实、密钥元数据、Outbox 都以数据库为最终事实源。
2. * * Redis 保存短生命周期、需要高频访问或原子竞争的运行时状态* *。Session、Refresh Token、Challenge 在当前实现中是 Redis 权威状态；Redis 丢失会导致会话失效，而不是从 MySQL
   自动恢复。
3. **进程内对象只保存可重建快照**。例如 JWKS 发布快照和 AuthZ 原生运行时；它们不能成为唯一业务事实。
4. * *跨资源一致性不伪装成单一事务* *。MySQL 内使用事务；MySQL 到 MQ 使用 Outbox；Redis 内使用 Lua 或 WATCH；MySQL 到 Redis 的身份状态传播使用专用 revocation
   outbox 和幂等 worker。

## 3. 事实源层次

```text
MySQL 持久业务事实
  ├─ users / profiles / login_identities / credentials
  ├─ roles / assignments / role_inheritances / permission_grants / resources / policy_versions
  ├─ jwks_keys 元数据
  ├─ domain_event_outbox
  └─ identity_session_revocation_outbox

Redis 运行时权威状态或派生缓存
  ├─ session / refresh_token / consumed_refresh_token / revoked_access_token
  ├─ challenge / OTP gate / OTP quota
  ├─ IDP access token / 微信 SDK cache
  └─ user、login identity 到 session 的索引

进程内派生状态
  ├─ AuthZ immutable runtime snapshot（含内存角色图）
  ├─ JWKS publish snapshot
  └─ Suggest trie/index runtime
```

“缓存”这个名称不能直接推导丢失语义。`session` 和 `refresh_token` 虽然放在 Redis，却是当前会话生命周期的权威状态；JWKS snapshot 和 Suggest index 才是可重建的派生状态。

## 4. 设计阅读方法

每篇文档都按同一条推理链组织：

```text
问题与约束
  -> 当前设计决策
  -> 代码如何实现
  -> 并发、事务与失败语义
  -> 替代方案和未选原因
  -> 当前代价与已知边界
  -> 证据与 Verify
```

不要从组件名反推保证。例如：

- 使用 Redis 不自动意味着操作是原子的；要继续看 Lua、WATCH 或事务管道；
- 使用 Outbox 不自动意味着 exactly-once；当前保证是“提交不丢 + 至少一次发布倾向”，消费者仍需幂等；
- 使用内存角色图不意味着它是业务真相源；数据库中的 Assignment、RoleInheritance 和 PermissionGrant 才是；
- 使用 JWT 不意味着请求完全无状态；在线验证还会检查 Session、撤销标记和当前主体状态。

## 5. 与业务模块的关系

| 基础设施 | 主要服务模块 | 不应拥有的业务决策 |
| --- | --- | --- |
| MySQL repository/UoW | 全部模块 | 是否允许注册、绑定、授权 |
| Redis stores | AuthN、IDP、Suggest | 用户是否有效、权限是否允许 |
| Event catalog/outbox | AuthZ、AuthN SMS 等 | 事件应在什么业务时机产生 |
| Crypto/keyset/JWT | AuthN、IDP | 谁可以登录、Token claims 的业务含义 |
| AuthZ native runtime | AuthZ | 角色/权限的业务创建规则 |
| REST/gRPC server | 全部模块 | 领域校验和事务编排 |
| Readiness/metrics | 全部模块 | 用探针结果替代业务事实 |

## 6. 当前特别需要记住的边界

- production/release 模式默认 fail closed；显式允许的 degraded 启动只用于受控场景。
- 一般领域事件 Outbox 与 Identity Session revocation outbox 是两套存储和 worker，目的不同，不能混称为同一队列。
- AuthZ 写入在同一事务更新管理事实、递增 policy version、暂存版本事件；提交后本实例 reload，其他实例通过消息订阅 reload。
- Redis 的 `TxPipelined` 保证命令批量提交，但不等同于 WATCH 条件更新；Session 更新使用 WATCH，Refresh Token 和 Challenge 使用 Lua。
- `/healthz` 只表示进程存活；`/readyz` 才检查 MySQL、Redis、AuthN、JWKS、AuthZ、Suggest 和 session revocation backlog。

## 7. Verify

```bash
make docs-hygiene
make docs-facts
go test ./internal/pkg/architecture/...
go test ./internal/apiserver/infra/...
```

涉及真实 MySQL/Redis 的测试可能需要外部服务；未执行时应明确记录，不能用 SQLite/miniredis 单元测试代替生产兼容性证据。
