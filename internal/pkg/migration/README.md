# 数据库迁移

本目录包含数据库迁移文件和迁移工具。

## ⚠️ 重要说明：迁移是增量变更，不是启动时全量重置

**常见误解：每次启动都执行迁移，会不会覆盖线上数据？**

**答案：不会重复执行已经成功记录的版本。** 迁移使用 **版本控制机制**：

```text
┌─────────────────────────────────────────┐
│  schema_migrations 表（自动创建）        │
├─────────────────────────────────────────┤
│  version  │  dirty                      │
│  1        │  false   ← 已执行版本       │
└─────────────────────────────────────────┘

第 1 次启动 → 执行 v1 迁移 → 记录 version=1 ✅
第 2 次启动 → 检查 version=1 → 跳过（0 SQL 执行）✅
第 3 次启动 → 检查 version=1 → 跳过（0 SQL 执行）✅
新版本发布 → 检查 version=1 → 仅执行 v2 → 记录 version=2 ✅
```

**关键点**：

- ✅ 迁移是**增量的**，不是全量的
- ✅ 每个版本只执行**一次**
- ✅ 后续启动会**跳过**已执行的版本
- ⚠️ 新的 forward migration 仍可能回填、修改或删除数据，必须逐版本审阅并先完成备份/恢复演练

## 旧库升级：children/guardianships 到 profiles/profile_links

2026-05 的 Identity 重构把运行时表切到 `profiles` / `profile_links`。已经执行过旧版
`000001` 的数据库物理表仍是 `children` / `guardianships`，不会因为后来修改了
`000001_init_schema.up.sql` 自动变化。

因此 `000002_add_profile_links_profile_id_index` 同时承担桥接职责：

- 若 `profiles` / `profile_links` 不存在，先创建 v2 运行时表；
- 若旧表 `children` 存在，把儿童档案复制到 `profiles`；
- 若旧表 `guardianships` 存在，把监护关系复制到 `profile_links`，旧 `guardian` 关系映射为 v2 的 `other`；
- 当时不删除 `children` / `guardianships`，保留作审计和回滚依据；
- 再幂等补充 `idx_profile_id` 索引。

后续单独授权的 `000019_retire_legacy_tables` 完成 Identity contract 阶段：

- 仅处理 `children` 和 `guardianships`，不夹带 AuthN、tenant、字典或审计表；
- canonical 表缺失时先创建，并用 `INSERT IGNORE` 只补缺失记录，不以历史快照覆盖当前数据；
- 对主键、关键字段、关系、撤销/软删除状态做一致性断言，并检查 FK、trigger、view、routine、event 依赖；
- 所有断言通过后才用一条最终 `DROP TABLE IF EXISTS children, guardianships` 删除旧表；
- down migration 明确不可逆，只能恢复经过验证的备份，不能伪造空旧表。

`000019` 的仓库与本地 MySQL 8 测试通过不等于生产已执行。生产发布仍要先备份和隔离恢复，使用只读预检确认依赖/对账，并在迁移后核对 `schema_migrations(version=19, dirty=0)`、canonical 行数与旧表不存在。

`000023_retire_legacy_authn_tables` 是后续独立授权的 AuthN contract：只删除
`auth_accounts/auth_credentials_legacy`，在最终原子 DROP 前重新断言 canonical schema、
format v5 已覆盖的数据映射和数据库对象依赖。缺少旧账号、因历史 JOIN 本就不可达的
凭据由发布前验证过的完整备份承接；down 明确 fail closed，不创建无法还原事实的空表。
生产已验收 `version=23, dirty=0` 且两张旧表不存在；canonical 待补数据在发布前为 0，
因此发布后不恢复旧表、不重新 merge runtime-unreachable 历史凭据。

如果某个环境已经用旧的 `000002` 启动失败，`schema_migrations` 可能处于
`version=2, dirty=1`。只有在确认失败点是“`profile_links` 不存在导致旧 `000002`
创建索引失败”，且没有手工执行过新 `000002` 的部分语句时，才可以在备份后把版本恢复为
`version=1, dirty=0`，再使用包含本修复的镜像重新启动迁移。

## 📁 目录结构

```text
migration/
├── migrate.go              # 迁移工具实现
├── migrations/             # 迁移 SQL 文件（嵌入到二进制）
│   ├── 000001_init_schema.up.sql      # 初始化表结构
│   ├── 000001_init_schema.down.sql    # 回滚表结构
│   ├── 000005_bootstrap_system_data.up.sql   # 最小系统初始化数据
│   ├── 000005_bootstrap_system_data.down.sql # 回滚最小系统初始化数据
│   ├── 000019_retire_legacy_tables.up.sql     # 退役 Identity 历史表
│   ├── 000019_retire_legacy_tables.down.sql   # fail-closed，不伪造回滚
│   ├── 000023_retire_legacy_authn_tables.up.sql   # 退役 Account-bound AuthN 历史表
│   ├── 000023_retire_legacy_authn_tables.down.sql # fail-closed，恢复依赖完整备份
│   └── ...
└── README.md               # 本文件
```

## 🚀 快速开始

### 1. 安装依赖

```bash
# 添加 golang-migrate 依赖
go get -u github.com/golang-migrate/migrate/v4
go get -u github.com/golang-migrate/migrate/v4/database/mysql
go get -u github.com/golang-migrate/migrate/v4/source/iofs

# （可选）安装 CLI 工具，用于创建迁移文件
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 2. 在应用中使用

```go
package main

import (
    "database/sql"
    "fmt"
    
    "github.com/FangcunMount/iam/v2/internal/pkg/migration"
    _ "github.com/go-sql-driver/mysql"
)

func main() {
    // 1. 连接数据库
    db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/iam")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // 2. 配置迁移器
    cfg := &migration.Config{
        Enabled:  true,              // 启用自动迁移
        Database: "iam",   // 数据库名称
    }

    // 3. 创建迁移器并执行
    migrator := migration.NewMigrator(db, cfg)
    if version, applied, err := migrator.Run(); err != nil {
        panic(err)
    } else if applied {
        fmt.Printf("migrated to version %d\n", version)
    } else {
        fmt.Printf("database already up to date (version %d)\n", version)
    }

    // 4. 启动应用...
}
```

### 3. 创建新的迁移

```bash
# 使用 migrate CLI 创建迁移文件
migrate create -ext sql -dir internal/pkg/migration/migrations -seq add_new_feature

# 生成的文件:
# - 000003_add_new_feature.up.sql   (升级脚本)
# - 000003_add_new_feature.down.sql (回滚脚本)
```

编辑生成的文件：

**000003_add_new_feature.up.sql**:

```sql
-- 添加新功能
ALTER TABLE iam_users ADD COLUMN nickname VARCHAR(64) COMMENT '昵称';
```

**000003_add_new_feature.down.sql**:

```sql
-- 回滚新功能
ALTER TABLE iam_users DROP COLUMN nickname;
```

## 🔧 高级用法

### 手动控制迁移

```go
// 获取当前版本
version, dirty, err := migrator.Version()

// 回滚最近的一次迁移
err = migrator.Rollback()
```

### 环境变量配置

```yaml
# configs/apiserver.prod.yaml
mysql:
  host: ${MYSQL_HOST:127.0.0.1}
  port: ${MYSQL_PORT:3306}
  database: ${MYSQL_DATABASE:iam}
  username: ${MYSQL_USER:root}
  password: ${MYSQL_PASSWORD:}

migration:
  enabled: ${MIGRATION_ENABLED:true}
```

### Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# 构建（迁移文件会被嵌入）
RUN go build -o /apiserver ./cmd/apiserver

# 运行阶段
FROM alpine:latest
COPY --from=builder /apiserver .
CMD ["./apiserver"]
```

容器启动时会自动执行迁移，无需手动操作。

## 📊 迁移表

`golang-migrate` 会自动创建 `schema_migrations` 表来追踪版本：

```sql
mysql> SELECT * FROM schema_migrations;
+---------+-------+
| version | dirty |
+---------+-------+
|       2 |     0 |
+---------+-------+
```

- `version`: 当前数据库版本号
- `dirty`: 是否处于中间状态（0=正常，1=异常需手动修复）

## 🔐 安全注意事项

### 生产环境

1. **备份优先**

   ```bash
   # 迁移前自动备份
   mysqldump iam > backup_$(date +%Y%m%d_%H%M%S).sql
   ```

2. **权限分离**
   - 应用账号：只需 SELECT, INSERT, UPDATE, DELETE
   - 迁移账号：需要 CREATE, DROP, ALTER 等 DDL 权限

3. **测试迁移**
   - 在测试环境先验证
   - 确保 down 脚本能正确回滚

4. **金丝雀发布**
   - 先在一个实例上执行
   - 验证成功后再推广

### 开发环境

```bash
# 重置数据库到初始状态
cd scripts/sql
./reset-db.sh

# 应用会在启动时自动执行迁移
go run cmd/apiserver/apiserver.go
```

## 📚 参考文档

- [golang-migrate 官方文档](https://github.com/golang-migrate/migrate)
- [数据库迁移指南](../../../docs/DATABASE_MIGRATION_GUIDE.md)
- [MySQL schema 与 bootstrap 事实源](../../../configs/mysql/README.md)
- [Bootstrap 数据](../../../configs/mysql/bootstrap.sql)

## ❓ 常见问题

### Q: 为什么要使用迁移工具？

A: 在容器化环境中，应用只打包二进制文件。使用 `embed.FS` 可以将 SQL 文件嵌入到二进制中，启动时自动执行，无需挂载外部文件。

### Q: 如何处理 dirty 状态？

A: Dirty 状态表示迁移中途失败。需要：

1. 检查日志确定失败原因
2. 手动修复数据库到一致状态
3. 确认失败版本的 SQL 是否已经部分生效
4. 只在数据库状态与目标版本一致时，才更新 `schema_migrations`

```sql
SELECT version, dirty FROM schema_migrations;
```

不要只把 `dirty` 改成 0 后重启。对于本轮已知的旧 `000002` 失败，如果确认失败发生在
`CREATE INDEX idx_profile_id ON profile_links` 且 `profile_links` 当时不存在，可以在备份后把
`schema_migrations` 恢复到 `version=1, dirty=0`，让修复后的 `000002` 重新执行。

### Q: 生产环境如何禁用自动迁移？

A: 设置环境变量：

```bash
export MIGRATION_ENABLED=false
```

### Q: 如何在 Kubernetes 中使用？

A: 使用 Init Container：

```yaml
initContainers:
- name: migrate
  image: your-app:latest
  command: ["/app/migrate-only"]  # 特殊命令只执行迁移
```

或者让应用启动时自动执行（推荐）。
