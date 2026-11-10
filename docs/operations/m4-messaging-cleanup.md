# M4 消息维护与旧装配清理

> 状态：已实现 · 2026-10-26 当前源码入口清理；合并、镜像与生产分别记账。

当前服务及授权维护写入只使用原`iam.mysql`事务owner。`m3-messaging`、`m3-messaging-recovery`及旧`reliable-messaging`预查入口直接拒绝，不读计划、环境或数据库；不再提供重新选择旧provider或回写旧schema的操作入口。

本批退出旧切换CLI实现及专用`messagingcutover`包、无生产调用的Relay/PolicyWirePublisher和两个M6一次性Go探针。原schema、迁移、SDK制品、已发布源码、业务/消息历史及保护备份都保留；需要审阅旧切换证据时使用原冻结材料，不能把旧CLI重新引入生产。

| 旧证明 | 当前职责及替代验证 |
| --- | --- |
| 旧离线安装/启动前恢复CLI及两项原生测试 | M3已完成，当前schema/read-only head、事务绑定与维护root仍通过原生门禁；旧命令由[无副作用拒绝测试](../../cmd/iam-maintenance/retired_messaging_test.go)保护。最终部署/恢复边界由infra C08重新冻结 |
| ReliableRuntime/PolicyWirePublisher测试 | 旧扫描发布器不再装配。当前owner的[生命周期测试](../../internal/apiserver/container/platform/transaction_lifecycle_test.go)、原gRPC/提交关闭测试及infra真实owner/原事务门禁保留 |
| M6一次性Go探针 | 对应旧shell入口已拒绝启动；当前IAM双实例/QS fanout与事务Checker由infra真实依赖证明，不宣称保留M6旧交接能力 |

首次空库bootstrap、历史迁移保留检测及身份到Session撤销仍有独立职责。不能因旧名或非MQ发布用途删除这些实现；授权维护工具的指纹、receipt、补偿和当前原事务归属保持。

当前产生事件的维护命令要求`--outbox-mode transaction --transaction-messaging-config <受限JSON路径>`；旧值在构造stager前拒绝。只读预查不触发发送，最终业务/部署证据由[infra M4总表](https://github.com/FangcunMount/infra/blob/main/docs/server/rocketmq-m4-execution.md)维护。

本批只修改源码、测试和文档；无生产部署、DDL、数据/备份删除、业务或模型调用。源码CI中的旧切换测试由精确的退役拒绝验证替代，当前schema、规划/维护原事务与并发门禁保持，缺依赖不能算通过。

## M5 标准 Outbox 历史读取候选

事务运行时启动前检查、Outbox 状态和历史审计可读取 `rm_outbox`，也可在原表缺席后读取固定的 `iam_rm_outbox_history`。两者只能存在一个可见的物理表。视图、缺失、权限不足和历史列缺失均阻断相关读取。历史表中仍有非终态时，事务启动保持拒绝。就绪检查立即失败，状态以 history_ 前缀继续报告这些记录。SDK 写入资格仍要求原表。历史审计标明来源，并保留原行分类及不授予执行权限的边界。

正式退役仍需独立审查。在兼容版本上线并验证后，单独签收实际账户的两对象读取权限、旧写者排空、原表 DDL 和逐行字节、备份与两地校验、前向启动及原名恢复路线。历史表候选由受审窗口原子改名得到，保留原数据和索引。源码和常驻启动均不安装该改名。 `domain_event_outbox`、事务见证、通知历史和调度留在各自范围。

## M5 旧通知 Outbox 历史读取候选

`domain_event_outbox` 的交接、就绪与只读审计支持保留原名，或读取固定的 `iam_domain_event_outbox_history`。归档读取要求原名已确认缺席、历史表全部记录列可读。双表、视图、缺失或权限不足时拒绝读取。发现未发布状态时启动仍拒绝，就绪报告保留对应旧通知分类。只读审计列出两份 Outbox 的实际来源，使用归档名时旧通知写入回退资格为 false。

迁移历史仍保留。阶段 33/35 的替代通知通过事务 owner 交付，需在受控版本中与本兼容读取一起验证。原名仍是旧通知写者所需的表。正式窗口单独确认实际账号权限、旧写者排空、原表 DDL 与逐行字节、两地备份及原名恢复。事务见证、标准 Outbox、通知历史与调度均保留。
