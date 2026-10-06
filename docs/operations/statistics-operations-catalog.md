# 统计运营目录配置维护

> 状态：已实现 · 2026-10-06 对照 statistics-operations、目录校验和标准维护写入修订。

## 配置结论

`qs:statistics:collection:operations/read` 表达运营计数访问。测评运营员、计划管理员、结果评估员获得该动作；内容管理员不新增权限，管理员继续沿用已有通配。实际范围由贡献该动作的 Assignment Scope 和 QS 有效 Operator 共同约束，目录动作不扩大公司/门店范围。

## 预演与写入

环境使用 `IAM_APISERVER_MYSQL_*`，沿用维护工具 USERNAME/USER、DATABASE/DBNAME 兼容；完整事实及指纹保存在私有新报告，不输出主体明细。预演列出待补目录、缺少此 Grant 的角色、已有 GrantID、完整事实 fingerprint 和 PolicyVersion。

```sh
iam-maintenance statistics-operations preflight --report /private/statistics/preflight-new.json
iam-maintenance statistics-operations apply --fingerprint REVIEWED_FINGERPRINT --actor-id REVIEWED_ADMIN_ID --writes-stopped --outbox-mode standard --event-catalog configs/events.yaml --report /private/statistics/apply-new.json
```

维护窗口暂停授权写入，重读并核对 fingerprint；actor 必须为有效受保护管理员。当前变更须显式 standard 模式，数据库 clean migration 39 或更新、标准列齐备且旧 Outbox 排空。还须核对运行 writer/Relay，门禁不会暂停它们。

## 原子性、幂等与限制

事务锁定版本并复核完整事实，只创建缺少目录和三个读取 Grant，原子递增 PolicyVersion 并写 Outbox。已有等价授权复用；已配置状态用当前预演指纹重跑不写入。目录动作冲突、角色保护异常或指纹漂移停止，不覆盖后续授权。

不新增/更新 Assignment 或 Scope。配置结果与本实例/全部实例快照收敛分开记录；QS 统计发布、普通账号真实列表/计数及跨公司/未归属拒绝另外验收。历史运行状态不能代替当前预演。

## 事实来源与验证

`cmd/iam-maintenance/operations_catalog.go`、`maintenance/operationscatalog/tool.go` 定义参数及计划；`infra/mysql/eventoutbox/maintenance_stager.go` 定义现行通知写入。tool_test.go 与真实 MySQL 专项覆盖复用、幂等、漂移和事务写入，业务统计场景由 QS 单独取证。
