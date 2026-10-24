# IAM M3 离线安装入口（验证中）

M3唯一总表由infra的`docs/server/rocketmq-m3-execution.md`维护。本组件属于T25-C2，不代表C2完成或生产批准；不修改业务写链、SDK版本和worker预算。

`iam-maintenance m3-messaging`不会启动API、SDK、Redis、MQ、Relay或业务自主任务。默认是只读预查，直接SELECT/SHOW，不构造可能创建`schema_migrations`的迁移驱动；它将私有原主库身份、真实版本表DDL摘要、已有新增对象DDL/是否非空、当前绑定/协议及两套旧Outbox责任写入新0600计划。不能覆盖既有计划；公开输出只有计划摘要和`cutover_authorized=false`。

两个独立步骤：

1. `--action schema --store-id <冻结store> --writer-epoch <冻结epoch> --export-plan <新0600文件>`仅预览。独立审核原文件摘要后，`--apply --writers-excluded --plan <原文件> --reviewed-sha256 <独立摘要>`显式运行41–45到固定45（生产起点40时包含原41审计表）；不自动绑定/选新协议。初始40/41及清洁的42–44可重新冻结各自计划，dirty拒绝，不force或向后迁移。
2. 安装后重新生成`--action select`计划并独立审核，再使用同样apply形式。要求两套旧Outbox无非published行，七张新增表除唯一初始outbox协议行外为空。在一个原MySQL事务写`iam.mysql`绑定和CAS选择`transaction/m3-1/generation=2`，不覆盖原绑定，不重新选择/重置已有事务库。提交回复未知保持Unknown，不自动重试。

连接复用原maintenance的`IAM_APISERVER_MYSQL_*`和TLS校验；本命令独占并关闭自有连接池，迁移驱动关闭自己的池不会关闭持有具名互斥的独立连接。GET_LOCK只串行化此离线入口，**不排除业务写者**。`--writers-excluded`只是操作者声明，不是停写证明、启动屏障或生产授权。实际调用必须放在唯一协调目录的受审pre-start-maintenance步骤中，实际目标持有原部署锁，按T29/T30冻结清单暂停授权写HTTP/全部维护工具/自动发布并排空旧链路。不能运行普通部署脚本来代替本入口。

保护候选范围是七个新增协议/业务证据对象、起点40额外新增的审计表和原真实`schema_migrations`，保留AuthN/login/session revocation事实在该范围外。当前入口**不创建保护备份、不执行删除/恢复**，不证明原库备份可恢复；保护/恢复附件和全体旧写者排除仍由C2/T29组合门禁完成。2026-10-25只读核验生产实际版本为40/clean；该起点必须先通过当前工具的40升级/恢复原生验证及新候选镜像演练，不能用41夹具或源码head替代。首个新runtime启动命令之后仍禁止旧备份覆盖，遵守infra不可逆启动屏障。

本地仅运行纯/race/编译/lint。`TestM3MessagingOfflineNative`仅GitHub隔离服务运行，缺依赖失败、不skip：使用随机临时schema及最小41夹具验证实际CLI只读预览、拒绝脏版本/错误主库/摘要/新责任、离线DDL、双旧账本排空及绑定/协议共同提交。AuthN保留行在预查后变化且保持，不能据此声称完整生产AuthN/全量恢复验收。

## 限定原范围保护/恢复（后续组件验证中）

`iam-maintenance m3-messaging-recovery --store-id <冻结store> --writer-epoch <冻结epoch> --export-protection <新0600文件>`只接受清洁40或41、七张M3新增表尚不存在且两套旧Outbox已排空的原库。40还须证明`iam_message_failure_audit`不存在，并将该第八个新增对象纳入独立保护范围；41的既有审计表及其中事实保持范围外。工具保存真实版本表DDL/行及原绑定身份；不复制AuthN或授权业务正文。保护原文件另取独立摘要，不能覆盖或从摘要推导生产批准。

只有唯一协调入口确认整次切换尚未发出任何新runtime启动命令、全体旧写者/在途DDL确实排除后，才允许受审`pre-start-restore`脚本调用`--restore --writers-excluded --pre-start-restore-attested --protection <原文件> --reviewed-sha256 <独立摘要>`。两个布尔参数只是声明，不是自动验证或恢复授权，不能独立手工调用来绕过协调屏障。

恢复持有本工具原锁和真实golang-migrate同名锁，重新读取精确主库、真实版本表DDL和原起点至45范围内的当前行；外键检查保持开启，不导入SQL文件。任何请求、执行、witness、业务operation或原生notice（包括终态）非空均拒绝；仅允许精确初始binding/协议元数据。只移除本保护证明原先不存在的七个空对象（起点40额外包含原先不存在且仍为空的审计表），再用原根CAS恢复原40或41/clean记录，版本表本身不DROP/重建；不会执行保护文件内的DDL字符串，不触及范围外业务事实。旧Outbox新责任、绑定矛盾、触发器、DDL变化、迁移仍运行或已有新事务都阻断。

DDL与版本行恢复不构成一个数据库事务；任何超时/中断/提交未知保持操作关闭，需要重新核对精确范围及独立受审处置，不能声称恢复完成。恢复成功会由协调屏障关闭本次新启动资格；不能拿同一文件重新开另一轮切换。当前仅本地纯/race/编译/lint，原库/原CLI及部分DDL实际恢复仍待隔离门禁；不勾选C2或生产验收。

离线42–45步骤后续改用同一冻结迁移文件的显式context入口：每一步SQL及版本行CAS都使用调用方deadline，先提交dirty再执行DDL，未知/取消不能清dirty；竞争原golang-migrate锁但不构造其带background context的通用驱动。原应用启动Migrator不变。补充真实版本表锁等待的250ms预算拒绝测试，实际门禁通过前不声称完成。

## 事务 runtime 启动

事务 owner 配置要求显式 `migration.enabled=false`；Options 及数据库初始化入口在创建连接前拒绝自动 DDL。关闭 DDL 仍使用原业务池以 2 秒预算只读核对唯一、clean 且与镜像内迁移 head 一致的真实版本行，不构造可能创建 metadata 的迁移驱动。缺库/缺 journal、脏行、旧/未来版本或多行均阻断。绑定、业务协议及 SDK schema 校验继续由原启动器完成；本检查不替代离线迁移或最终生产身份核验。

40起点新增表只复用既有000041原迁移，不改写旧迁移；离线计划的模板摘要现包含41–45，旧摘要/保护文件必须重新导出和独立审核。SDK codec/schema/稳定tag及业务写入语义不变；仍无生产执行授权。
