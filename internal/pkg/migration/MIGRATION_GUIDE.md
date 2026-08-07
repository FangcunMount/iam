# 数据库迁移入口

> 状态：当前入口。旧版 guide 使用不存在的 `iam_*` 表名、失效 Make target 和过时 AuthN 模型，已于 2026-08-07 退役；不要从 Git 历史复制其中的 SQL 执行。

IAM 使用 `golang-migrate`。Schema 的唯一事实源是同目录 `migrations/*.sql`，由 `migrate.go` 通过 `embed.FS` 加载，版本和 dirty 状态只记录在 `schema_migrations`。

## 当前规则

- 已发布 migration 不原地修改；修复只新增 forward 版本。
- 应用启动负责执行 migration；没有当前 `make db-migrate`、`make db-version` 或 `make db-rollback` 入口。
- `configs/mysql/bootstrap.sql` 只在 schema 达到当前版本后写幂等系统基线数据，不创建表，也不能替代 migration。
- destructive down 不提供虚假空表回滚；`000019–000020` 明确 fail closed，恢复依赖已验证备份。
- 历史表退役必须先执行 secret-safe 只读 preflight，仓库 migration 存在不等于目标库已经执行。

## 权威文档

- 当前迁移和发布机制：`docs/05-工程质量与运维/03-迁移发布与数据库运维.md`
- MySQL 与事务边界：`docs/03-基础设施/01-MySQL事务与迁移.md`
- 遗留表退役台账：`docs/05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md`

验证当前迁移合同：

```bash
go test ./internal/pkg/migration -count=1
python3 scripts/check-docs-facts.py
```
