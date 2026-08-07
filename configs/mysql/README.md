# MySQL 初始化材料

IAM 的 schema 唯一事实源是 `internal/pkg/migration/migrations/*.sql`。迁移文件通过 `go:embed` 编入应用，并由 `internal/pkg/migration` 按 `schema_migrations` 版本顺序执行。

本目录只保留 `bootstrap.sql`：它是 schema 已到达当前迁移版本后可重复执行的系统基线数据，不创建、不修改、也不删除表结构。`make db-bootstrap` 只是该数据脚本的便捷入口，不能替代迁移。

自 `000021_retire_unused_platform_tables` 起，bootstrap 不再维护 `tenants` 主数据或 `data_dictionary` 字典；AuthZ 使用的 `platform`、`fangcun` tenant domain 继续直接保存在角色、分配、策略版本和 Casbin policy 中，不依赖已退役的平台辅助表。

约束：

- 不从静态 schema 快照初始化数据库；
- 不修改已经发布的迁移，结构修复必须新增 forward migration；
- 先启动 IAM 完成迁移并确认 `schema_migrations` clean，再运行 bootstrap；
- bootstrap 只允许数据写入语句，不允许 `CREATE TABLE`、`ALTER TABLE` 或 `DROP TABLE`；
- AuthZ tenant domain 的数据库 canonical 值是 `fangcun`，历史数值 token 的兼容由认证归一层承担。
