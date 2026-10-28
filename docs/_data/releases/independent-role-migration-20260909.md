# 独立业务角色迁移执行记录

> 状态：已实现 · 生产迁移与继承表退役已执行；最终授权隔离与业务验收继续按下文验证。

## 数据依据

2026-09-09 通过 IAM 维护入口只读查询生产 IAM 与 QS。预演覆盖 129 位用户，原直接分配中 staff 124、evaluator 2；迁移目标 operator 125、reviewer 118（按用户合并，不是新增分配数量）。原始逐人报告保存在执行主机受限目录，不将人员身份列表提交仓库。预演没有发现身份映射异常；真正切换前必须冻结写入并重新预演，不能复用本文人数作为执行依据。

## 兼容批次

- IAM `iam-maintenance role-model-migrate status|preflight|apply|verify|rollback|archive-inheritance`，使用 MYSQL_* 与 QS_MYSQL_* 环境配置。完成态检查只需 IAM 连接；首次预演和迁移才需 QS。连接信息不得放在命令参数或报告中。
- 生产入口：GitHub Action `Role Model Migrate`（[原 role-model-migrate 工作流](https://github.com/FangcunMount/iam/blob/8f297a77520151d2789671d912bf18cd52e833eb/.github/workflows/role-model-migrate.yml)（2026-10-28 已退役；历史执行记录保留））。QS 复用同一 MYSQL HOST/PORT/USERNAME/PASSWORD，另配 Secret `QS_MYSQL_DBNAME`。完整报告在主机 `/opt/backups/iam/database/role-model/`。
- schema 33 仅增加迁移清单表；不会自动变更人员权限。
- Apply / archive 必须 `--writes-stopped` 与一致的 `--fingerprint`；cutover 模式在 Action 内于 preflight 成功后自动串联，不把人员明细打进工作流日志。
- QS 最终版本始终检查后台专业查询的动作权限与业务范围，不依赖环境开关。进度查询独立返回 DTO。临床身份探测不依赖岗位标记。

## 最终清理批次（本分支）

- 运行时仅解析直接 Assignment；`roles` 与 `direct_roles` 同值。
- 删除继承领域、仓储、图遍历与管理 handler；旧 REST 路径返回 `410 Gone`。
- `archive-inheritance` + migration `000034` 归档并删除 `authz_role_inheritances`。
- 新库完整链经 fresh stage 直接得到独立角色目标态。

## 切换顺序与验收

1. 首次迁移按兼容批次顺序发布；已完成迁移的环境先执行 status，不重新发放角色。核查全部配置消费者以及继承 CRUD 使用情况。
2. 暂停后台业务、授权写入及 QS 人员身份变更；建议先 `Database Operations` → `backup`。跑 `Role Model Migrate`：`preflight` 通过后再 `cutover`（`confirm=ROLE_MODEL_MIGRATE`，`writes_stopped=true`）。
3. cutover（含 archive）成功后，再 Production Deploy，让自动迁移执行 `000034` 删表；确认 `version=34, dirty=0`。
4. 发布始终检查权限的 QS 最终版本；更新 QS 人员角色投影，确认授权缓存、运行时策略版本收敛和 Outbox 无积压。
5. 验证新岗位的正反权限、条件 retry、医生关系范围、参与者自服务和结果查询缓存；通过后恢复后台服务。

Rollback 只能补偿未被后续授权写入修改的完整事实；冲突时停止。PolicyVersion 必须继续递增。应用查询边界与角色发放成组回滚。删表后须先恢复结构及归档数据，再恢复旧二进制。

## 已验证与未完成

IAM 最终清理相关单测已通过；迁移测试在 SQLite 和本机真实 MySQL 路径具备。运营端与 QS 兼容开关侧此前已验证。GHA 维护入口已落地。

2026-09-10 工作流 34433990157 完成 apply、verify、archive；34434140486 完成 IAM 部署。后续只读 status 返回 applied_unchanged、继承表不存在、归档存在。该证据不替代医生报告、条件 retry、逐用户权限及运行时收敛验收。

## 后续检查与安全重跑

`status` 输出 pending、applied_unchanged、applied_drifted、rolled_back 或 invalid，以及 next_action、原始 fingerprint、after_hash、current_hash、changed_sections。历史已完成与当前事实一致是两个不同结论。

- pending：首次预演仍核对 QS 医生身份及全部授权事实。
- applied_unchanged：重跑不修改角色、分配、策略版本或 Outbox；若归档步骤未完成，可以继续归档。
- applied_drifted：只报告变化，verify、apply、归档和自动回滚均不能绕过冲突保护；核对正常后续授权记录，不重置清单基准。
- rolled_back：同一迁移标识不自动重迁移。
- invalid：清单、归档或校验损坏，停止操作。

生产状态报告保存于受限目录。真实 MySQL 8 CI 专项必须配置 ROLE_MODEL_MYSQL_DSN，并设置 ROLE_MODEL_REQUIRE_MYSQL=true，禁止静默跳过归档与回滚测试。删表后的回滚先恢复原始 DDL 和归档行，再补偿授权事实并发布更高策略版本。
