# MySQL、Unit of Work 与数据库迁移

> 状态：已实现 · 已与 `internal/pkg/database/mysql`、三个模块 UoW、迁移 000001–000018 和相关测试核对。

## 1. 本文回答

- 为什么 IAM 把 MySQL 作为持久业务事实源？
- Repository、Unit of Work、数据库约束分别解决什么问题？
- 为什么“先查再写”不能独立保证唯一性？
- Identity、AuthN、AuthZ 的事务边界如何装配？
- 启动迁移的执行和失败语义是什么？
- 还有哪些可选设计，为什么当前没有采用？

## 2. 30 秒结论

IAM 采用“**领域/应用层定义端口，MySQL adapter 实现持久化，应用服务声明事务边界，数据库约束兜底并发不变量**”的组合。

```text
application use case
  -> module-specific UnitOfWork.WithinTx
    -> shared GORM UnitOfWork starts transaction
      -> tx handle stored in context
        -> repositories obtain tx through BaseRepository.WithContext
          -> database constraints provide final concurrency guard
```

这套设计的关键不在于 GORM，而在于三类责任被分开：

- 领域对象/领域服务表达业务规则；
- 应用服务决定一次用例包含哪些写入；
- 数据库决定并发时最终允许哪一个提交成功。

## 3. 要解决的问题

### 3.1 单仓储写入不等于业务事务

注册可能同时创建 `User`、`LoginIdentity`、`Credential`；授权变更可能同时写角色绑定或权限事实、递增 `PolicyVersion`、暂存 Outbox 事件；封禁用户需要更新 `users` 并暂存 Session 吊销任务。

如果每个 repository 自己提交，失败会留下半成品：

```text
User 已创建
LoginIdentity 创建失败
Credential 未创建
=> 数据结构合法，但业务用例只完成了一半
```

因此事务边界属于应用用例，不属于某个 repository。

### 3.2 应用检查不能消除并发竞争

`CheckPhoneUnique`、`SelfProfileGuard` 等检查能提供友好错误，也能承载领域语言；但两个请求可以同时读到“不存在”。最终唯一性必须由数据库唯一索引或锁保护。

当前典型双层保护：

| 不变量 | 应用/领域保护 | 数据库保护 |
| --- | --- | --- |
| active user phone 唯一 | `UniquenessChecker` | migration 000017 的 active phone generated column + unique index |
| 每个 User 最多一个 active self ProfileLink | `SelfProfileGuard` | migration 000007 的 active self guard |
| 只能有一个 active JWKS key | key lifecycle check | migration 000016 的 single-active guard |
| LoginIdentity provider key 唯一 | repository lookup/builder | `login_identities` 唯一索引 |

数据库错误再由 repository translator 映射为稳定业务错误，避免把 MySQL 1062 直接泄漏给上层。

## 4. Shared Unit of Work 如何工作

共享入口在 `pkg/uow/gorm`，`internal/pkg/database/mysql/uow.go` 只是 IAM 内部别名和适配层。

核心机制：

1. `WithinTransaction` 用 GORM 开启事务；
2. 当前 `*gorm.DB` transaction handle 写入 `context.Context`；
3. module UoW 在回调中用 `RequireTx(ctx)` 取出它；
4. 创建绑定到该 transaction handle 的 repository 集合；
5. 回调返回 error 时回滚，nil 时提交；
6. 可选 after-commit hook 只在提交成功后执行。

这种做法避免在每个 repository 方法参数里传 `*gorm.DB`，但也带来一个重要约束：事务内代码必须沿用传入的 `txCtx`。错误地改用外层 `ctx`，repository 会退回普通 DB handle，事务原子性就会被绕开。

`internal/pkg/architecture` 和 UoW 测试用于保护这条约束，但 code review 仍需检查 context 传递。

## 5. 三个模块的事务聚合

### 5.1 Identity

`internal/apiserver/application/identity/uow.TxRepositories` 聚合：

- `Users`
- `Profiles`
- `ProfileLinks`
- `SessionRevocations`

因此 `Deactivate`/`Block` 可以在更新用户状态的同一事务中写入 `identity_session_revocation_outbox`。MySQL 提交成功后，即使 Redis 暂时不可用，吊销意图仍可恢复。

### 5.2 AuthN

AuthN UoW 聚合注册所需的 `Users`、`LoginIdentities`、`Credentials`。外部 provider code exchange 在事务外完成，数据库写入在事务内完成。

把网络调用放在事务外是有意的：

- 避免长时间持有数据库锁；
- provider 超时不会占用事务连接；
- 先把外部 proof 规范化为 prepared input，再进入本地原子写入。

代价是外部 proof 解析和数据库提交之间存在时间窗口，所以 provider 返回值不能被当作已完成注册，只能作为事务输入。

### 5.3 AuthZ

AuthZ UoW 聚合：

- 角色/资源/绑定 repository；
- `casbin_rule` authorization fact store；
- `PolicyVersion` repository；
- `event.Stager`。

`PolicyChangeCommitter` 在一个事务里完成业务事实、Casbin 持久事实、版本递增和 Outbox 暂存。提交后才触发当前实例的 runtime reload。

这避免出现“内存 Enforcer 已允许，但数据库事务随后回滚”的反向不一致。

## 6. Repository 与映射职责

IAM repository 通常由三部分组成：

```text
domain entity/value object
  <-> mapper
persistence object (PO, GORM tags)
  <-> repository
MySQL table/index
```

Repository 应负责：

- 查询和保存；
- PO/领域对象映射；
- 驱动错误翻译；
- 必要的数据库锁和原子 SQL。

Repository 不应负责：

- REST/gRPC DTO；
- 跨仓储用例编排；
- 根据当前调用场景决定业务权限；
- 发布跨系统副作用。

`BaseRepository.WithContext` 会优先使用 context 中的事务 handle；这就是同一 module UoW 下多个 repository 共享事务的连接点。

## 7. 数据库锁、约束与幂等

### 7.1 悲观锁的使用

`UnlinkOwnedUnlessLastActive` 在 MySQL 中对用户的 LoginIdentity 集合加 `FOR UPDATE`，再计算 active 数量并更新目标。这把“不能解绑最后一个活跃登录身份”的检查和写入放在同一锁范围中。

它没有只依靠下面这种竞态写法：

```text
count active -> if count > 1 -> update
```

因为两个并发解绑都可能读到 2，然后把两个都删掉。

### 7.2 唯一约束不是可有可无的优化

唯一约束是业务并发安全的一部分。领域检查负责可读错误，数据库约束负责竞争裁决。删除约束会改变外部可观察行为，不能当作普通 schema 清理。

### 7.3 软删除与 active guard

部分表使用软删除；普通唯一索引可能阻止“历史记录保留但允许重新创建”。当前迁移通过 generated active column/guard 将唯一性限定在 active 行。文档和 repository 查询必须同时说明是否包含已撤销/软删除记录。

## 8. 迁移机制

迁移文件通过 `go:embed migrations/*.sql` 打进二进制，由 `golang-migrate` 执行。

启动路径：

```text
DatabaseManager.Initialize
  -> init MySQL/Redis registries
  -> runMigrations
    -> 独立 sql.DB
    -> 检查 dirty version
    -> migrate.Up
```

设计要点：

- 迁移使用独立 `sql.DB`，避免关闭 migrator 时影响业务 GORM 连接；
- dirty version 直接失败，需要人工核查，不能自动 force；
- up/down 必须成对，当前事实门禁要求最新版本为 18；
- 迁移失败会终止正常启动；MySQL 未配置时开发/显式 degraded 场景可跳过，但 release 模式随后会在资源/关键模块校验处 fail closed。

`configs/mysql/schema.sql` 和 `bootstrap.sql` 用于初始化/参考，增量演进的权威顺序是 migration 文件；二者冲突时必须先确认部署路径，不能只改一个副本。

## 9. 为什么不采用其他方案

### 9.1 每个 repository 自己开事务

未采用。它无法覆盖跨仓储业务用例，嵌套事务语义也会变得不清晰。

### 9.2 把 `*gorm.DB` 暴露给 application

未采用。这样 application 会依赖具体基础设施，测试替身和架构边界变差。当前用 module-specific UoW port 暴露最小事务能力。

### 9.3 只靠分布式锁保证唯一性

未采用。唯一性属于数据不变量，数据库约束更靠近事实源；分布式锁还会引入租约、脑裂和失败恢复问题。

### 9.4 所有模块共用一个巨型 UoW

未采用。模块 UoW 只暴露当前用例需要的 repository，避免 AuthN application 随意访问 AuthZ/IDP 表。共享实现只负责 transaction context，不负责扩大业务边界。

### 9.5 启动时自动修复 dirty migration

未采用。自动 force 可能把未完成 DDL 当作成功；IAM 选择停止并要求人工确定数据库真实状态。

## 10. 当前代价与风险

- context 携带 transaction handle 是隐式机制，调用者误用外层 context 会绕开事务；
- GORM PO hook 和 mapper 都可能参与字段同步，修改时要检查两处；
- SQLite 单测不能证明 MySQL generated column、`FOR UPDATE`、`SKIP LOCKED` 的真实语义；
- 自动迁移把发布和 schema 变更耦合，生产需要备份、权限和回滚流程；
- `schema.sql`、`bootstrap.sql`、migrations 三套材料存在维护放大，必须由 facts test/人工审计防漂移。

## 11. 证据入口

| 关注点 | 代码/契约 |
| --- | --- |
| shared UoW | `pkg/uow/gorm/uow.go`、`internal/pkg/database/mysql/uow.go` |
| repository transaction routing | `internal/pkg/database/mysql/base.go` |
| Identity UoW | `internal/apiserver/infra/mysql/uow/identity/uow.go` |
| AuthN UoW | `internal/apiserver/infra/mysql/uow/authn/uow.go` |
| AuthZ UoW | `internal/apiserver/infra/mysql/uow/authz/uow.go` |
| migration runner | `internal/pkg/migration/migrate.go` |
| migration SQL | `internal/pkg/migration/migrations/` |
| database bootstrap | `internal/apiserver/process/database.go` |
| duplicate translation | `internal/pkg/database/mysql/translator.go` |

## 12. Verify

```bash
make docs-facts
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./pkg/uow/gorm ./internal/pkg/database/mysql
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/pkg/migration/...
/Users/yangshujie/.gvm/gos/go1.25.12/bin/go test ./internal/apiserver/infra/mysql/uow/...
```

需要 MySQL 的 contract/concurrency 测试应单独报告环境与结果。

## 13. 面试追问

**为什么同时需要领域校验和数据库唯一索引？**

领域校验表达意图并给出业务错误；唯一索引裁决并发。前者不能关闭 TOCTOU 窗口，后者也不能解释业务语义。

**Unit of Work 与 Repository 的区别？**

Repository 抽象某类聚合的持久化；Unit of Work 抽象一次业务用例的提交边界，并让多个 repository 共享同一事务。

**Outbox 为什么也要放进 AuthZ UoW？**

只有事件意图与授权事实同事务提交，才不会出现“数据库成功但事件永久丢失”或“事件已发但数据库回滚”。
