# SQL Bootstrap 与初始化数据

本文回答：`iam` 现在如何管理“系统 bootstrap 真值”，`schema.sql`、`bootstrap.sql` 和 migration 各自负责什么，以及为什么仓库已经不再把 `seeddata` 作为现行初始化入口。

## 30 秒结论

- `seeddata` 是 zero 阶段的历史 bootstrap 工具，当前已经退出现行主路径。
- 今天最可信的初始化入口有两层：
  - `internal/pkg/migration/migrations/*.sql`：应用启动时自动执行的 schema 与 bootstrap
  - `configs/mysql/bootstrap.sql`：需要人工重放时可直接执行的幂等初始化 SQL
- `configs/mysql/schema.sql` 现在只负责“完整表结构基线”，不再偷偷夹带系统 seed。
- `make db-bootstrap` 是对 `bootstrap.sql` 的手动入口，适合重建后补齐系统租户、基础用户/账号、角色资源策略和数据字典。

## 重点速查

| 关注点 | 当前答案 | 真实落点 |
| ---- | ---- | ---- |
| 表结构真值 | migration + schema 基线 | [../../internal/pkg/migration/migrations/](../../internal/pkg/migration/migrations/)、[../../configs/mysql/schema.sql](../../configs/mysql/schema.sql) |
| 系统 bootstrap 真值 | migration `000005` + `bootstrap.sql` | [../../internal/pkg/migration/migrations/000005_bootstrap_system_data.up.sql](../../internal/pkg/migration/migrations/000005_bootstrap_system_data.up.sql)、[../../configs/mysql/bootstrap.sql](../../configs/mysql/bootstrap.sql) |
| 手工重放入口 | `make db-bootstrap` | [../../Makefile](../../Makefile) |
| 现行是否还需要 seeddata | 不需要 | 本文结论 |

## 1. 为什么不再保留 seeddata 主路径

`seeddata` 当初存在的原因是：

1. 系统还处于 zero 阶段
2. 空库无法直接启动出一个可工作的最小系统
3. 需要一条“一次性造出 system init、基础账户、用户体系数据”的 bootstrap 工具链

现在这条前提已经变化了：

- 应用本身已经有稳定 migration 机制
- JWKS 也有运行时自动初始化路径
- 真正必须长期保留的，是系统 bootstrap 真值，而不是整套模拟账户、家庭、跨服务编排工具

因此继续把 `seeddata` 当现行入口，只会带来双轨维护成本。

## 2. 三类 SQL / migration 的职责

### 2.1 `schema.sql`

职责：

- 提供完整表结构基线
- 便于离线审阅、一次性建库、与 migration 对照
- 对需要手工执行 `bootstrap.sql` 的表保持与运行时 schema 兼容

不负责：

- 账户样例
- 家庭样例
- 历史 seed 场景
- 任何需要跨服务编排的数据造数逻辑

### 2.2 `bootstrap.sql`

职责：

- 承载系统 bootstrap 真值
- 保持幂等，可人工重放
- 让默认租户、基础用户/账号、系统角色、资源策略、默认微信应用、数据字典有一个稳定 SQL 入口

当前内容包括：

1. `fangcun` / `platform` 租户与联系信息
2. `system` / `admin` / `content_manager` 基础用户、运营账号和密码凭据
3. IAM / QS 的基线角色、资源目录、角色分配、Casbin 策略、策略版本
4. 默认微信应用元数据
5. 性别、用户状态、监护关系等基础字典

### 2.3 migration `000005`

职责：

- 把 `bootstrap.sql` 的系统 bootstrap 真值接入应用启动路径
- 保证 fresh DB 在自动迁移后就具备可登录、可授权、可对接默认微信应用的基础数据

这意味着：

- 手工执行 `bootstrap.sql` 是补救/重放入口
- migration `000005` 才是运行时真正的自动入口

## 3. 当前推荐流程

### 3.1 正常开发 / 部署

1. 启动 MySQL
2. 启动 `iam-apiserver`
3. 应用自动执行 migration
4. migration 自动补齐系统 bootstrap 基线数据

### 3.2 手工修复 / 重放系统真值

```bash
make db-bootstrap DB_USER=root DB_PASSWORD=yourpassword
```

适用场景：

- 环境重建后需要补齐系统 bootstrap 数据
- 某些基础账号、角色资源策略或字典被误删后需要重放
- 需要对比 migration 与离线 SQL 的效果

## 4. 当前边界

### 已完成

- `schema.sql` 与 bootstrap 数据职责已拆开
- `schema.sql` 已对齐 bootstrap 依赖的运行时认证表结构
- migration 已接入系统 bootstrap
- `Makefile` 与 README 已切出 `db-bootstrap` 入口

### 不再承诺

- 不再承诺提供家庭、儿童、监护关系等业务样例造数入口
- 不再把跨服务 testee 创建这类逻辑当作 IAM 主初始化面的一部分

## 5. 继续往下读

| 文档 | 说明 |
| ---- | ---- |
| [03-命令&契约校验与开发流程.md](./03-命令&契约校验与开发流程.md) | `Makefile` 里的 `db-bootstrap`、质量链与开发入口 |
| [04-端口&证书与数据库迁移.md](./04-端口&证书与数据库迁移.md) | migration 与部署口径 |
| [../../configs/mysql/schema.sql](../../configs/mysql/schema.sql) | 完整表结构基线 |
| [../../configs/mysql/bootstrap.sql](../../configs/mysql/bootstrap.sql) | 系统 bootstrap 真值 SQL |
