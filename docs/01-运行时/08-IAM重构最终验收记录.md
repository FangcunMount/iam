# IAM 重构与生产验收记录

> 状态：已实现 · 本文按截至 2026-08-26 的可复核证据记录迁移、发布与生产观察结果；未完成项不视为已验收。

## 1. 当前结论

截至 2026-08-26，生产库已验收 `version=27, dirty=0`，16 张 BASE TABLE 精确匹配，active RoleBinding 重复组为 0；`casbin_rule`、`authz_cutover_state` 与 `authz_resources.scope_kinds` 均不存在。AuthZ v3 功能基线 SHA `d3f58369d8c58dbf50ae15282f5641bc370055a6` 已部署并通过独立健康检查。`000019–000027` 的仓库事实、迁移发布和部署观察已经闭合；migration 25 基线 `c84c638d46ade0a2b1b65379289931ef9e28b172` 仍作为 RoleBinding guard 的历史发布证据保留。

当前 checkout 已新增 000028 AuthN `global_identifier` 唯一性迁移；本记录没有它的生产发布或只读验收证据，因此上述生产结论仍停留在 27，不能由仓库门禁结果外推为 28 已部署。

整体历史验收仍未完全关闭：完整镜像 digest 没有在本安全台账中单独固化，长期 RDS/宿主机备份策略以及 5.4 安全处置历史元数据仍有缺口。缺少的证据必须由对应 workflow 或运维记录产生，不能根据当前系统状态反推，也不能为了补记录再次清除 Refresh Token 或历史日志。

## 2. 安全记录规则

本文只登记 SHA、镜像 digest、发布时间、workflow run URL/ID、数量、字节、时间、operator、ticket 和通过/失败结论。禁止粘贴 Token、SQL、日志正文、Redis key、数据库地址或凭据。

## 3. 仓库侧证据

| 项目 | 状态 | 证据入口 |
| --- | --- | --- |
| AuthZ `Check` / `GetAuthorizationSnapshot` 安全错误 | 已实现 | `internal/apiserver/transport/grpc/service/authz/service.go`、`service_test.go`、`internal/pkg/architecture/architecture_test.go` |
| IDP 通用 `ExternalIdentity` 信任边界 | 已实现（仓库侧） | `domain/idp/externalidentity` 定义请求级值对象，`application/idp/externalidentity` 统一解析三类 provider；AuthN SignIn、SignUp、Linking 通过单一 capability 消费，Identity v2 同名 proto 仍是未接入的历史 transport 契约 |
| 数据库操作单一脚本 | 已实现 | `scripts/dbops/database-operation.sh`、`database_operation_test.go` |
| MySQL 8 合成备份恢复与 migration 25 guard | 已实现并完成同 SHA 验收 | [MySQL 8 run `32791721351`](https://github.com/FangcunMount/iam/actions/runs/32791721351) 覆盖 full-chain migration、复合唯一索引并发语义、生产同款 preflight 与 backup/restore fixture |
| 文档可生成事实门禁 | 已实现 | `scripts/check-docs-facts.py` 从 proto、bootstrap、开发配置和 active Markdown 生成期望值，校验服务矩阵、资源示例、Quick Start 端口和状态计数 |
| Active docs 语义分类 | 已生成核对 | Active docs 状态计数：总计 `78` 篇，`已实现` `78` 篇，`规划改造` `0` 篇。历史目标提示词已退出 active 层 |
| 遗留资产退役 | 已完成生产验收 | `000019–000024` 的批次证据见 [遗留资产、兼容层与数据库退役审计](../05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md) |
| AuthZ v3 一步到位切换 | 已完成生产验收 | [切换 `32859067799`](https://github.com/FangcunMount/iam/actions/runs/32859067799)、[数据库状态 `32876762969`](https://github.com/FangcunMount/iam/actions/runs/32876762969)、[RoleBinding guard `32876761874`](https://github.com/FangcunMount/iam/actions/runs/32876761874) |

## 4. 最终发布证据

以下条目绑定 migration 25 的 RoleBinding guard 历史发布基线 `c84c638d46ade0a2b1b65379289931ef9e28b172`。成功 run 只证明对应层，不自动补齐其他证据；AuthZ v3 最终切换证据另列在其后。

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| 最终源码 SHA | 已核验 | 发布负责人 | 当前部署 run、镜像 tag 与 `/version.gitCommit` 均绑定 `c84c638d46ade0a2b1b65379289931ef9e28b172` |
| 镜像 digest 与发布时间 | 部分核验 | 发布负责人 | [生产部署 run `32792113058`](https://github.com/FangcunMount/iam/actions/runs/32792113058) 于 2026-08-25 成功构建并部署同 SHA 镜像；完整 digest 尚未在本安全台账单独登记 |
| CI | 已通过 | 发布负责人 | [CI run `32791721343`](https://github.com/FangcunMount/iam/actions/runs/32791721343)，同一最终 SHA |
| CodeQL | 已通过 | 发布负责人 | [CodeQL run `32791721416`](https://github.com/FangcunMount/iam/actions/runs/32791721416)，同一最终 SHA |
| MySQL 8 workflow | 已通过 | 发布负责人 | [MySQL 8 run `32791721351`](https://github.com/FangcunMount/iam/actions/runs/32791721351)，同 SHA 的空库完整升级、并发 guard 与生产同款 preflight 均成功 |
| Production Deploy | 已通过 | 平台运维 | [生产部署 run `32792113058`](https://github.com/FangcunMount/iam/actions/runs/32792113058)，Build/Deploy/Summary 三个 job 均成功 |
| Production Health Check | 已通过 | 平台运维 | [健康检查 run `32792503728`](https://github.com/FangcunMount/iam/actions/runs/32792503728) 证明容器 healthy、`/healthz=200`、`/readyz=200`、运行 SHA 精确匹配且 MySQL/Redis 可达 |
| AuthZ gRPC 安全消息 | 仓库侧通过，生产抽样待登记 | 应用运维 | 安全 mapper 与测试已通过；仍需保存不含底层错误文本的生产黑盒验证结论 |

### 4.1 AuthZ v3 最终切换

| 项目 | 当前状态 | 证据 |
| --- | --- | --- |
| 生产数据库转换与退役 | 已通过 | [run `32859067799`](https://github.com/FangcunMount/iam/actions/runs/32859067799)：备份 `iam_backup_20260825_222411.sql.gz` 后完成 000026、105 条 Grant、6 条继承、hash 对账和 000027 |
| 生产数据库最终状态 | 已通过 | [run `32876762969`](https://github.com/FangcunMount/iam/actions/runs/32876762969)：`version=27, dirty=0`、16 张 BASE TABLE，旧授权表/列持续缺席 |
| 最终 IAM 部署 | 已通过 | [run `32877019508`](https://github.com/FangcunMount/iam/actions/runs/32877019508)：SHA `d3f58369d8c58dbf50ae15282f5641bc370055a6`，ACR 镜像 digest `sha256:98599f7aced99218…` |
| 最终 IAM 健康 | 已通过 | [run `32877567211`](https://github.com/FangcunMount/iam/actions/runs/32877567211)：运行 SHA 精确匹配、容器 healthy、health/readiness 200、MySQL/Redis 可达 |
| Mac mini 生产备份克隆演练 | 已通过 | [run `32875449928`](https://github.com/FangcunMount/iam/actions/runs/32875449928)：完整 25→27 链、转换 hash、最终旧对象缺失和 artifact checksum 均通过 |

## 5. 数据库备份证据

生产数据库为阿里云 RDS MySQL 8.0。RDS 原生备份负责实例级恢复，IAM `Database Operations` 负责从生产宿主机生成可移植的单库逻辑备份；两条链路分别验收，不能互相替代。

### 5.1 RDS 原生备份

证据来源为平台运维于 2026-07-27 提供的生产 RDS 控制台基础备份列表和备份策略截图。截图未纳入仓库，记录中省略实例 ID、连接地址及其他非必要标识。

| 项目 | 当前状态 | 已核验事实或剩余边界 |
| --- | --- | --- |
| 实例与基础备份 | 已核验 | MySQL 8.0 实例运行中；所示最近 5 个全量实例快照均为“备份完成”，最新一份于 2026-07-27 05:16:53 完成并形成恢复时间点 |
| 快照策略 | 已核验 | 周日、周一、周三、周五 05:00 执行，保留 7 天；不是每日备份，最长计划间隔约 48 小时 |
| 日志与秒级备份 | 部分通过 | 日志备份已开启并保留 7 天，秒级备份已开启；控制台“任意时间点恢复”仍为关闭，不得宣称已具备 PITR |
| 实例释放后保护 | 未完成 | 实例释放后不保留备份文件；如需防止误释放导致最后恢复点丢失，需另行启用至少保留最后一份 |
| 跨地域灾备 | 未完成 | 跨地域备份关闭；当前证据不覆盖地域级故障 |

RDS 基础备份满足当前最低保护要求，但增强恢复能力只完成一部分。PITR、实例释放后保留和跨地域灾备是否启用，需由平台运维根据 RPO、RTO、成本和地域级风险另行决策；在决策完成前以上边界必须保留在验收记录中。

### 5.2 IAM 宿主机逻辑备份与恢复门禁

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| `000019–000024` 发布前逻辑备份 | 已按批次登记 | 平台运维 | 文件元数据、run URL 与发布对应关系见 canonical [退役审计台账](../05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md) |
| 当前定时数据库状态检查 | 已通过 | 平台运维 | [Database Operations run `32876762969`](https://github.com/FangcunMount/iam/actions/runs/32876762969) 证明 `version=27, dirty=0`、16 张 BASE TABLE 精确匹配且全部退役对象持续缺席 |
| 长期宿主机定时逻辑备份 | 部分闭合 | 平台运维 | [backup run `32756675723`](https://github.com/FangcunMount/iam/actions/runs/32756675723) 产生 `iam_backup_20260825_012703.sql.gz`，29,210,688 bytes，`gzip -t=pass`且保留数为 3；长期 RPO/RTO 与隔离恢复演练仍需持续登记 |
| 最终 SHA 的 MySQL 8 合成恢复 | 已闭合 | 发布负责人 | [MySQL 8 run `32791721351`](https://github.com/FangcunMount/iam/actions/runs/32791721351) 完成同 SHA 的 backup -> drop -> restore -> data assertion |

## 6. 5.4 安全处置历史证据

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| Refresh Token purge | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、dry-run matched 数量和结果；不得再次执行 purge 来制造历史证据 |
| 修复前 IAM 日志处置 | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、文件数量、总字节和结果；不得读取或复制正文 |
| 一个 Access Token TTL 观察 | 未完成：历史证据尚未在仓库中登记 | 应用运维 | 登记观察起止时间和“无新增 Token/SQL/raw gRPC error 泄露”的结论 |

历史证据确实无法取得时，本记录保持未完成，并由安全负责人决定是否另开维护窗口；本轮不默认授权任何破坏性操作。

## 7. 关闭条件

当前可以分别陈述“仓库门禁通过”“`000019–000027` 生产退役完成”“AuthZ v3 最终 SHA 已部署且健康检查成功”，不能合并成“所有历史安全与恢复事项已最终关闭”。只有第 4、5、6 节剩余项都有真实、安全的元数据证据，才能把整个 IAM 历史治理结论改为“最终验收全部完成”；这些历史缺口不改变 AuthZ v3 切换已经完成的结论。
