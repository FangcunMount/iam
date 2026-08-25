# Redis、运行时状态与缓存一致性

> 状态：已实现 · 已与 cache family catalog、AuthN/IDP Redis adapters、Lua/WATCH 实现和治理接口核对。

## 1. 本文回答

- IAM 为什么使用 Redis，哪些数据是缓存，哪些是权威运行时状态？
- String、ZSet、Lua、WATCH 和 TTL 分别解决什么问题？
- Session、Refresh Token、Challenge 的并发语义是什么？
- Redis 故障时系统如何失败？
- 为什么当前只提供 inspect-only 缓存治理？

## 2. 30 秒结论

IAM 不是把 Redis 当成一个泛化 `map[string]any`，而是先登记 13 个稳定 cache family，再为每个 family 明确：owner、key pattern、数据角色、Redis 类型、编码、TTL 来源、写入和失效方式。其中 Suggest 的 Redis/进程内限流是互斥可选 family，治理面只把实际启用的后端计入运行状态。

最关键的区分是：

| 类型 | 例子 | Redis 丢失的后果 |
| --- | --- | --- |
| 权威运行时状态 | Session、Refresh Token、用户/登录身份 Session 索引 | 在线会话失效，需要重新登录；当前没有 MySQL 恢复源 |
| 撤销/挑战标记 | revoked access token、Challenge、OTP gate/quota | 安全语义可能暂时变弱或请求失败，因此正常运行依赖 Redis 可用 |
| 远端 token 缓存 | 微信 access token、微信 SDK cache | 重新向 provider 获取，可能放大流量 |
| 派生内存快照 | JWKS publish snapshot | 可从 MySQL key metadata 重建 |

因此 `/readyz` 把 Redis 设为 required，release 模式不会把 Redis 缺失当作普通 cache miss 继续服务。

## 3. Cache Family 是治理契约

静态目录位于 `internal/apiserver/cache/catalog.go`。它不是 Redis client wrapper，而是回答：

```text
谁拥有这类数据？
它是事实、marker、远端缓存还是派生快照？
key 和 value 的稳定形态是什么？
TTL 由谁决定？
如何写入和失效？
治理面允许做什么？
```

当前治理能力只有 `inspect`。这是一个安全边界：缓存清空、重建和迁移会改变在线认证状态，不能因为“有缓存管理 API”就默认允许 destructive mutation。

## 4. 13 个 family 的设计

| Family | 后端/类型 | 角色 | 关键一致性机制 |
| --- | --- | --- | --- |
| `authn.refresh_token` | Redis String(JSON) | 权威状态 | Lua 原子轮换，key TTL |
| `authn.consumed_refresh_token` | Redis String(JSON marker) | 重放检测标记 | `sha256(token)` key；TTL=旧 token 原剩余寿命；只保存 Session/User 引用 |
| `authn.revoked_access_token` | Redis String(marker) | 撤销标记 | 存在性检查，TTL=token 剩余时间 |
| `authn.session` | Redis String(JSON) | 权威状态 | WATCH 乐观事务，TTL=Session expiry |
| `authn.user_session_index` | Redis ZSet | 权威索引 | member=sid，score=expiry，懒清理 |
| `authn.login_identity_session_index` | Redis ZSet | 权威索引 | member=sid，score=expiry，懒清理 |
| `authn.challenge` | Redis String(JSON) | 短期 marker | Lua compare-and-delete/attempt counter |
| `authn.login_otp_send_gate` | Redis String(marker) | cooldown | `SET NX` + TTL |
| `authn.login_otp_send_quota` | Redis ZSet | 滑动窗口 | Lua 计数、租约、失败补偿 |
| `idp.wechat_access_token` | Redis String(JSON) | 远端 token cache | 分布式 refresh lock |
| `idp.wechat_sdk` | Redis String | SDK cache | 调用方 TTL |
| `authn.jwks_publish_snapshot` | process memory | 派生快照 | 重建覆盖，短时复用 |
| `suggest.redis_rate_limit` | Redis String(marker) | 限流计数 | `INCR` + 首次 `EXPIRE`，多副本共享 |
| `suggest.memory_rate_limit` | process memory | 限流桶 | 单进程令牌桶，容量淘汰 |

## 5. 为什么使用不同数据结构

### 5.1 String：单对象、整对象读取、独立 TTL

Refresh Token、Session、Challenge 都按唯一 key 寻址，通常整对象读取和替换。String(JSON) 让 key 级 TTL 和 compare-and-set 脚本直接成立。

不用 Redis Hash 的原因不是 Hash 不可行，而是当前没有频繁的局部字段更新需求；Hash 的 field 更新会增加 codec、版本兼容和 TTL 管理复杂度。

### 5.2 ZSet：按过期时间维护反向索引或滑动窗口

Session 需要按 user/login identity 批量吊销。只保存 `session:{sid}` 无法高效枚举，因此增加两个 ZSet：

```text
user_session_index:{userID} -> (sid, expiresAtUnix)
login_identity_session_index:{loginIdentityID} -> (sid, expiresAtUnix)
```

score 是过期时间，读取前用 `ZREMRANGEBYSCORE` 懒清理。这避免为索引 key 单独维护精确 TTL，但意味着索引和主对象是两个数据结构，写入/撤销必须同步维护。

OTP quota 用 ZSet 保存窗口内发送租约，可在 Lua 中完成清理、计数、添加和设置 TTL。

## 6. 原子性：pipeline、Lua、WATCH 不能混为一谈

### 6.1 `TxPipelined`

Session 创建把主对象和两个索引放入事务 pipeline，减少中间可见窗口。但 pipeline 不提供“只有旧值仍等于 X 才更新”的条件判断。

### 6.2 Lua

Refresh Token 轮换要求：只有旧 token 仍存在且 token ID 匹配时，才能写入新 token 并删除旧 token。Lua 在 Redis 单线程执行模型中把比较、写新、删旧合为一个原子步骤。

```text
GET old
  -> token_id 匹配？
    -> SET new PX ttl
    -> DEL old
```

并发刷新时只有一个请求成功，其他请求得到 `ErrRefreshTokenNotFound`。这不是普通 500，而是 replay/conflict 的安全结果。

Challenge 成功消费也用 compare-and-delete Lua；失败次数脚本只给当前 secret version 计数，Challenge 被替换后旧请求不会污染新 challenge 的 attempt counter。

### 6.3 WATCH

Session revoke/extend 要读取对象、判断状态、修改 payload 并维护索引，逻辑比简单 compare-and-delete 更复杂，因此使用 WATCH + retry。版本冲突最多重试 5 次，并线性 backoff；耗尽后返回明确 optimistic transaction conflict。

## 7. Session 主对象与索引一致性

### 7.1 创建

`Save` 同时：

- `SET session:{sid}`；
- `ZADD user_session_index:{userID}`；
- `ZADD login_identity_session_index:{loginIdentityID}`。

### 7.2 延期

`Extend` WATCH 主对象，确认仍 active，更新 `ExpiresAt` 和主 key TTL，并刷新两个 ZSet score。

### 7.3 撤销

`Revoke` WATCH 主对象，写入 revoked 状态（或已过期时删除），同时从两个索引移除 sid。

### 7.4 批量吊销

按 user/login identity 读取 ZSet，逐个加载 Session 并调用单 Session revoke。主对象缺失时会清理 stale member。

当前语义不是单 Lua 脚本一次原子吊销全部 Session。批处理中途 Redis 出错可能已吊销前一部分，调用者/worker 需要重试；单 Session 撤销幂等使重试可行。

## 8. MySQL 到 Redis 的跨存储一致性

User block/deactivate 既要修改 MySQL 身份事实，又要吊销 Redis Session。MySQL 与 Redis 没有共同事务。

当前设计：

```text
MySQL transaction
  ├─ update users.status
  └─ insert identity_session_revocation_outbox
commit
  -> Identity worker claim task
  -> AuthN SessionRevoker.RevokeByUser
  -> complete or retry task
```

这选择了最终一致性，而不是在数据库事务内直接调用 Redis。原因：Redis 成功后 MySQL 回滚会误杀会话；Redis 失败又会让数据库事务无法可靠补偿。

登录、refresh 和在线 verify 同时执行 `AdmissionPolicy`，通过 Identity 的 `UserStatusReader` 与 AuthN 的 LoginIdentity Repository 读取当前状态。这是“异步吊销窗口”内的同步安全门；revocation worker 负责清理存量 Session。

## 9. IDP access token 的防击穿

微信 access token 是远端资源，不是 IAM 用户 token。缓存 miss/临近过期时，多个实例同时 refresh 会放大 provider 调用并可能互相覆盖。

`AccessTokenCacher` 采用：

1. 读取缓存并检查带 skew 的有效期；
2. 尝试获取 refresh lock；
3. 获得锁者调用 provider 并写缓存；
4. 未获锁者短暂等待后重读；
5. provider/cache 错误向上返回，而不是伪造 token。

替代方案 singleflight 只能协调单进程，不能覆盖多实例；因此当前需要 Redis lock。它仍不是强一致分布式锁，锁 TTL 与 provider timeout 必须协调。

## 10. 故障语义

| 故障 | 当前结果 | 原因 |
| --- | --- | --- |
| Redis 启动不可达（release） | readiness/启动关键资源失败 | 会话和认证状态依赖 Redis |
| Session key 丢失 | 在线验证/刷新失败，需重新登录 | 当前无 MySQL session 恢复源 |
| 索引 member 残留 | 批量吊销时懒清理 | 索引可自修复部分 stale member |
| 主对象存在、索引缺失 | 按 user 批量吊销可能漏掉该 Session | 需要写路径测试/治理观察，当前无全量重建器 |
| Refresh Lua 冲突 | 仅一方成功，其他视为 token 已消费 | 防 replay |
| Challenge Lua 冲突 | 仅首个正确消费者成功 | 防 OTP 重放 |
| IDP cache miss | 尝试加锁并回源 | 远端 token cache 可重建 |

## 11. 为什么不采用其他方案

### 11.1 所有状态都写 MySQL

未采用。Session/Challenge/OTP quota 是高频、短 TTL、强过期语义数据；MySQL 可实现但清理、锁竞争和延迟成本更高。

### 11.2 JWT 完全无状态，不保存 Session

未采用。IAM 需要 logout、用户封禁、登录身份禁用、refresh rotation 和在线撤销；完全无状态 JWT 只能等待过期或维护另一套 denylist，实质仍需要状态。

### 11.3 只用进程内 map

未采用。多实例不共享，重启全丢，无法支持一致的 OTP/Session 竞争。

### 11.4 用分布式事务覆盖 MySQL 和 Redis

未采用。复杂度和可用性成本远高于当前需求；专用 outbox + 幂等 worker 更容易观测和恢复。

### 11.5 暴露通用“按前缀清缓存”接口

未采用。误删 Refresh Token/Session 是用户可见的安全操作。当前治理面只读 inspect，清理需要独立运维流程和安全确认。

## 12. 当前代价与风险

- Session 主对象与两个 ZSet 不是单个 Redis key，存在极端部分写入/外部误删导致的索引漂移；
- Redis 是单一运行时依赖，持久化/AOF/RDB 和灾难恢复策略属于部署责任；
- Lua 使用 JSON 字符串 needle 比对 token/secret 字段，修改序列化格式时必须同步脚本和测试；
- 批量 Session 吊销逐个处理，不是全批原子操作；
- inspect-only 能看配置/健康，但不能自动证明每个主对象都有完整索引；
- cache family catalog 与实际 key builder 需要 facts test/代码审计持续防漂移。
- Suggest 限流错误仍按既有策略 fail-open；治理面现在能标记实际 Redis limiter 不健康，但它不替代告警与保护策略决策。

## 13. 证据入口

| 关注点 | 代码 |
| --- | --- |
| family catalog | `internal/apiserver/cache/catalog.go` |
| governance read service | `internal/apiserver/application/cachegovernance/service.go` |
| Refresh Token | `internal/apiserver/infra/cache/redis/token-store.go` |
| Session 与索引 | `internal/apiserver/infra/cache/redis/session_store.go` |
| Challenge | `internal/apiserver/infra/cache/redis/challenge_repository.go` |
| OTP gate/quota | `internal/apiserver/infra/cache/redis/otp_verifier.go` |
| IDP token cache | `internal/apiserver/infra/cache/redis/accesstoken_cache.go` |
| Identity revocation outbox | `internal/apiserver/infra/mysql/sessionrevocation/` |

## 14. Verify

```bash
go test ./internal/apiserver/cache/...
go test ./internal/apiserver/application/cachegovernance/...
go test ./internal/apiserver/infra/cache/redis/...
go test ./internal/apiserver/infra/mysql/sessionrevocation/...
```

miniredis 可以验证命令和竞态契约，但不能替代真实 Redis 的持久化、集群和故障切换验证。

## 15. 面试追问

**Redis 里的 Session 到底是不是缓存？**

按部署组件它常被泛称缓存；按数据语义它是当前会话权威状态。判断标准应是“丢失后能否从其他事实源无损重建”，不是存储产品名称。

**Lua 和 WATCH 怎么选？**

固定少量命令、适合 compare-and-set 的操作用 Lua；需要读取复杂对象、执行 Go 领域判断并更新多个结构时用 WATCH + retry。两者都必须定义冲突结果。

**为什么 AdmissionPolicy 不能被异步 Session 吊销替代？**

异步 worker 存在传播窗口且可能积压；同步检查当前 User/LoginIdentity 状态关闭安全窗口，worker 负责回收存量会话和减少后续成本。
