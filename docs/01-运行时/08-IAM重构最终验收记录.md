# IAM 重构与生产验收记录

> 状态：已实现 · 本文按 2026-08-18 可复核证据记录仓库、迁移、发布与生产观察结果；未完成项不视为已验收。

## 1. 当前结论

截至 2026-08-18，当前生产基线 `eebb5b4acca19ffb15414750ec47a7bc534be207` 已有同 SHA 的 CI、CodeQL、生产部署和持续健康检查成功证据；`000019–000024` 也已分别完成生产退役验收。仓库事实、迁移退役和当前部署观察已经闭合。

整体历史验收仍未完全关闭：同一最终 SHA 没有单独触发 MySQL concurrency workflow，完整镜像 digest 没有在本安全台账中单独固化，长期宿主机逻辑备份以及 5.4 安全处置历史元数据仍有缺口。缺少的证据必须由对应 workflow 或运维记录产生，不能根据当前系统状态反推，也不能为了补记录再次清除 Refresh Token 或历史日志。

## 2. 安全记录规则

本文只登记 SHA、镜像 digest、发布时间、workflow run URL/ID、数量、字节、时间、operator、ticket 和通过/失败结论。禁止粘贴 Token、SQL、日志正文、Redis key、数据库地址或凭据。

## 3. 仓库侧证据

| 项目 | 状态 | 证据入口 |
| --- | --- | --- |
| AuthZ `Check` / `GetAuthorizationSnapshot` 安全错误 | 已实现 | `internal/apiserver/transport/grpc/service/authz/service.go`、`service_test.go`、`internal/pkg/architecture/architecture_test.go` |
| 数据库操作单一脚本 | 已实现 | `scripts/dbops/database-operation.sh`、`database_operation_test.go` |
| MySQL 8 合成备份恢复 | 已实现门禁，最终 SHA 待补运行 | `.github/workflows/concurrency-tests.yml`；最近成功 run 为 [`31600598524`](https://github.com/FangcunMount/iam/actions/runs/31600598524)，对应前一 SHA `7e501470…` |
| 文档可生成事实门禁 | 已实现 | `scripts/check-docs-facts.py` 从 proto、bootstrap、开发配置和 active Markdown 生成期望值，校验服务矩阵、资源示例、Quick Start 端口和状态计数 |
| Active docs 语义分类 | 已生成核对 | Active docs 状态计数：总计 `76` 篇，`已实现` `76` 篇，`规划改造` `0` 篇。历史目标提示词已退出 active 层 |
| 遗留资产退役 | 已完成生产验收 | `000019–000024` 的批次证据见 [遗留资产、兼容层与数据库退役审计](../05-工程质量与运维/06-遗留资产兼容层与数据库退役审计.md) |

## 4. 最终发布证据

以下条目绑定当前已部署基线 `eebb5b4acca19ffb15414750ec47a7bc534be207`。成功 run 只证明对应层，不自动补齐其他证据。

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| 最终源码 SHA | 已核验 | 发布负责人 | 当前部署 run 与健康检查均绑定 `eebb5b4acca19ffb15414750ec47a7bc534be207` |
| 镜像 digest 与发布时间 | 部分核验 | 发布负责人 | [生产部署 run `31602684018`](https://github.com/FangcunMount/iam/actions/runs/31602684018) 于 2026-08-12 成功构建并部署同 SHA 镜像；完整 digest 尚未在本安全台账单独登记 |
| CI | 已通过 | 发布负责人 | [CI run `31602268673`](https://github.com/FangcunMount/iam/actions/runs/31602268673)，同一最终 SHA |
| CodeQL | 已通过 | 发布负责人 | [CodeQL run `31602270081`](https://github.com/FangcunMount/iam/actions/runs/31602270081)，同一最终 SHA |
| MySQL 8 workflow | 未闭合 | 发布负责人 | 最后成功 run `31600598524` 对应 `7e501470…`；需在要求“同一最终 SHA”时显式补跑并登记 |
| Production Deploy | 已通过 | 平台运维 | [生产部署 run `31602684018`](https://github.com/FangcunMount/iam/actions/runs/31602684018)，Build/Deploy/Summary 三个 job 均成功 |
| Production Health Check | 已通过 | 平台运维 | [健康检查 run `32092344087`](https://github.com/FangcunMount/iam/actions/runs/32092344087)，2026-08-18 对当前 SHA 的容器检查和报告均成功 |
| AuthZ gRPC 安全消息 | 仓库侧通过，生产抽样待登记 | 应用运维 | 安全 mapper 与测试已通过；仍需保存不含底层错误文本的生产黑盒验证结论 |

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
| 当前定时数据库状态检查 | 已通过 | 平台运维 | [Database Operations run `32050037012`](https://github.com/FangcunMount/iam/actions/runs/32050037012) 对当前 SHA 成功；status 成功不等于产生新备份 |
| 长期宿主机定时逻辑备份 | 未闭合 | 平台运维 | 仍需登记 backup run ID、文件数量、总字节、目录 `0700`、文件 `0600` 和 `gzip -t=pass` |
| 最终 SHA 的 MySQL 8 合成恢复 | 未闭合 | 发布负责人 | 需显式运行 concurrency workflow，并登记同 SHA 的 backup/restore 成功 run URL |

## 6. 5.4 安全处置历史证据

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| Refresh Token purge | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、dry-run matched 数量和结果；不得再次执行 purge 来制造历史证据 |
| 修复前 IAM 日志处置 | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、文件数量、总字节和结果；不得读取或复制正文 |
| 一个 Access Token TTL 观察 | 未完成：历史证据尚未在仓库中登记 | 应用运维 | 登记观察起止时间和“无新增 Token/SQL/raw gRPC error 泄露”的结论 |

历史证据确实无法取得时，本记录保持未完成，并由安全负责人决定是否另开维护窗口；本轮不默认授权任何破坏性操作。

## 7. 关闭条件

当前可以分别陈述“仓库门禁通过”“`000019–000024` 生产退役完成”“当前 SHA 已部署且健康检查成功”，不能合并成“所有历史安全与恢复事项已最终关闭”。只有第 4、5、6 节剩余项都有真实、安全的元数据证据，且最终 SHA 的 MySQL 8 恢复门禁补齐后，才能把整体结论改为“最终验收全部完成”。
