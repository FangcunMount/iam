# 授权维护操作入口

> 状态：已实现 · 2026-10-06 按当前维护工具整理；命令说明不构成一次生产执行或验收证明。

先确认维护对象、当前清单状态、生效版本、输入/指纹和恢复产物，再进入对应操作。授权事实、业务身份与人工验收分别留证，历史记录不替代当前读取。

| 任务 | 操作正文 | 独立边界 |
| --- | --- | --- |
| 旧分配配置公司/门店范围 | [Assignment Scope 迁移](assignment-scope-migration.md) | IAM/QS 两库无原子事务；历史上线记录单列 |
| 创建登录身份 | [维护身份准备](account-provision.md) | 不自动产生 Operator、Role 或 Scope |
| 固定独立审核账号 10002 | [10002 专用流程](reviewer-10002-provisioning.md) | 已知 Actions Scope 写入缺口；人工审核不自动执行 |
| 配置运营统计读取目录 | [统计目录维护](statistics-operations-catalog.md) | 不修改分配或数据范围 |
| 维护 ACL 跨部署保留 | [授权写入冻结](authz-write-freeze.md) | 不自动暂停 REST、任务或 QS 配置写入 |
| 条件历史数据 | [条件退役维护](../02-业务模块/03-AuthZ/08-条件授权退役维护手册.md) | 当前 Runtime 不能恢复非空条件 |

动作/Scope 模型与版本失效合同归 [AuthZ 模块](../02-业务模块/03-AuthZ/README.md)。当前授权维护写入统一显式 standard Outbox，schema/排空门禁与旧 writer/Relay 实际停止分开核对；详细运行所有权归事件与发布正文。
