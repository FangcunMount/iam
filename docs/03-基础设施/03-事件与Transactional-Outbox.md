# 事件、Outbox 交接与投递结果

> 状态：已实现 · 依据 IAM 当前装配及锁定 reliable-messaging v0.5.0 / go-nsq v1.1.0；源码条件、离线测试和运行接受分别标明。

## 当前生产装配：显式事务owner

M4 C02-01收敛[进程准备](../../internal/apiserver/process/event_bus.go)和[InitEventing](../../internal/apiserver/container/platform/eventing.go)：仅接受显式RocketMQ、唯一 `iam.mysql` owner及旧 `events.reliable_messaging.enabled=false`。默认/显式NSQ、无owner的普通消息Relay或混合装配都在catalog、数据库或Broker准备前拒绝。启动不创建Topic、订阅或ACL，资源由infra管理。

授权事实、精确版本、操作journal与witness沿原PlannedUnitOfWork提交；Checker、CAS、幂等、双订阅和freshness保持。NORMAL只承担原尽力通知；可选MQ SMS仍由事务owner持有独立NORMAL客户端。`publish_enabled=false`封新准入，保留Checker及原关闭顺序，不启动发布型Outbox扫描。

当前RocketMQ失败审计仍使用 `iam_message_failure_audit`。`identity_session_revocation_outbox`承担Redis撤销责任，历史表的只读未决检查也继续保留；名称带Outbox不代表发布Relay仍运行，不在本批删表/删卷。未使用的旧SDK helper、历史维护入口及格式清理由M4 C03/C04/C05分别收口。

旧生产装配专用的三个wiring/NSQ业务/四进程交接夹具已退出。替代证明为平台/进程的依赖前拒绝、最终装配无I/O构造、M3原事务原子性/操作journal、原生Broker及IAM→QS fanout；旧wiring/业务/image-handoff脚本明确拒绝，不再构建已移除的装配。当前RocketMQ失败审计继续由独立真实MySQL工作流验证；固定源码、CI和部署状态见[infra M4总表](https://github.com/FangcunMount/infra/blob/main/docs/server/rocketmq-m4-execution.md)。

## 已退出的普通消息Relay（历史设计）

原M1曾以普通消息配合发布型Outbox和NSQ备选。其运行装配已删除，旧证明脚本返回退役错误；不能通过关闭事务配置恢复该路径。以下保留的原事务说明和维护历史不授予旧Relay运行或重新发布权限。

## M3 独立事务模式候选

固定Go稳定版`v0.5.0`包含先前修订预发布的修复：Broker指标回写与ACK hook并发访问meter的race；公开事务API、wire、schema及原结算语义不变。原事务和HTTP验收必须使用下载的正式模块，源码合并或SDK发布不代表IAM生产切换。

事务模式显式配置 `messaging.transactions.iam.mysql`、version2 路由和 `configs/events-m3.yaml`，关闭标准 Outbox 的 enabled 开关；两种写模式不能同时启用。`events.reliable_messaging.shutdown_timeout` 继续提供宿主退出预算，独立于旧 Outbox 是否开启。SDK、连接池和 Checker 的生命周期由宿主管理，公开幂等 API 与生产切换仍按 [infra M3 总表](https://github.com/FangcunMount/infra/blob/main/docs/server/rocketmq-m3-execution.md)验收。

迁移43安装 `iam_authz_message_protocol`，默认 `outbox`，不会自动转换 provider。事务选择要求精确 owner/store/epoch/route_revision；原根事务先取得 execution guard，再持有协议行共享锁，最后共同提交业务、精确版本和 witness。标准 Stager 在原根事务中同样校验门禁，拒绝选择事务后的旧 Outbox 写入。缺表仅在有干净且精确单行的旧迁移日志时允许旧模式；新版迁移缺门禁失败关闭。旧二进制没有此门禁，生产仍须停止全部旧写者并验证排空，不能由一行配置代替交接。

只读规划先核验选择，完整冻结 ID、时间和首次 wire 后才准备半消息。全部半消息确认成功后，单次宿主执行器开启原根事务。回调返回 nil 不是提交证明；根提交 ACK 或权威 witness 才可证明本地提交，最终 Broker 确认丢失保留 `LocalCommitted + FinalUnknown`。取消也不证明回滚，不自动重做业务。

进程在事务 owner 启动失败时不降级启动。drain 先封闭新规划及 NORMAL 准入，让真实在途调用在 Checker 存活时结束，再关闭客户端和宿主资源；未完成退出保留 `ShutdownIncomplete`。required readiness 增加 `message_transaction`，不再用空旧 Outbox 证明事务 owner 已就绪。临时写暂停与读取可用性分开，关闭后的实例不能重新开放准入。

### 业务操作凭证与 HTTP 幂等候选

迁移44显式安装独立 `iam_authz_operations`，不复用旧 Outbox 状态。原请求先竞争完整冻结计划，赢家才 Reserve SDK；原 SQL 根事务同时持有 SDK guard 与业务操作 guard，提交授权事实、精确版本、类型化首次结果和 witness。重复请求先读取原结果并重新核对当前权限，不重做业务；同键不同稳定业务输入返回409。无实际修改的操作保存首次结果及版本，不发送半消息，并通过版本行锁校验原只读规划仍有效。

事务模式的 REST 授权写接口接受可选 `Idempotency-Key`，按已验证主体、原作用域和操作类型隔离，长度1–255字节、禁止空白边界和歧义多值；不带键保留旧调用，分配本次操作身份但不承诺跨请求去重。成功正文维持原格式，额外返回 `Operation-ID` 与 `Operation-Location`。旧 Outbox 模式收到键返回503，不能静默接受一个无效承诺。已证实本地提交保留原成功语义；未知结果返回503和原操作引用，已中止返回409，不换键自动重试。

`GET /api/v4/authz/operations/:operation_id` 需要合法登录、原主体、原作用域及当前原操作的全部权限（包含原受保护角色权限）。当前 REST 写接口仅管理全局授权，查询只开放其 `global` 记录；组织 claim 本身不构成组织权限，组织作用域的服务间记录不通过此 REST 入口开放。状态仅含公开业务结果、版本和生产者三阶段分类，不含 wire、凭据、execution token、幂等键或 Broker 身份。数据库错误和权限版本落后均不能沿用旧 ALLOW。这里的提交状态不证明所有订阅者收敛。

维护 CLI 保持受信任 OS 主体和显式预检/停写合同，不新增跨命令幂等键；拒绝注入 HTTP 风格键。其操作记录只保存公开结果投影，完整 before/after 恢复材料留在原私有维护证据中。无论回查或中止维护都不重新执行维护命令。

同 owner 的扫描预算由 SDK 与业务凭证共用（默认各50条/5秒，总计100条），不增加业务 worker 配额。查询、宿主规划和在途数据库维护都计入关闭等待；超时保留未完成责任，Checker 和宿主连接池不能被当作已排空提前关闭。以上为当前源码合同，合并、固定矩阵、真实 Broker 和生产/system 验收分别在 infra 总表记账。

## 1. 当前设计先保护哪一个窗口

授权版本事件使用RocketMQ事务消息：先冻结计划、准备全部半消息，再进入宿主原MySQL根事务，在同一根提交授权事实、精确PolicyVersion、操作结果及witness。任一半消息失败或未知不启动本地业务。SDK不接管业务连接池，不重复执行授权命令；没有发布型Outbox扫描或NSQ降级路径。

| 链路 | 当前机制及责任 |
| --- | --- |
| AuthZ version变更 | `iam.mysql` TRANSACTION；IAM自身与QS独立订阅分别收敛 |
| OTP等尽力通知 | NORMAL直接发布；保留原尽力语义，不当作事务成功凭证 |
| Identity到Session撤销 | 原独立session-revocation持久化及worker；不属于本次MQ发布型Outbox退役范围 |

[装配入口](../../internal/apiserver/container/platform/eventing.go)在资源使用前拒绝NSQ及旧发布Outbox配置；[事务owner](../../internal/apiserver/container/platform/transaction_eventing.go)要求精确`iam.mysql`、`m3-1`目录及路由，并借用原主库。构造不做网络/DB调用、DDL或启动后台任务。完整预算、只读材料及实际部署以infra冻结契约为准。

## 2. 三阶段结果和原事务

半消息、原业务根提交、Broker最终确认分别记录。`LocalCommitted + FinalUnknown`保留业务已提交事实，不返回假回滚、不重新执行命令。进程退出后由同owner的新实例读取精确原证据完成Checker；取消、lease到期、缺行或DB错误本身不是回滚证明。

[PlannedUnitOfWork](../../internal/apiserver/infra/mysql/uow/authz/planned.go)在原事务重验权限、业务前置及版本CAS。ID、时间和wire在事务外冻结，重复输入复用原计划，不重新生成密文。Committed witness、业务操作journal及当前版本共同提交；回查只读权威证据，不能授权业务重试。无变化操作不制造虚假版本事件。

生产者本地提交与消息最终一致，不表示消费者已处理。IAM reload/轮询、QS自身订阅、授权freshness、消费幂等与会话撤销继续各负其责。

## 3. 准入、Checker与关闭

临时关闭新写保持Checker存活；空闲实例也登记完整事务Topic。协议维护与业务journal共用原扫描预算，不额外放大worker并发。预检核对原绑定、schema及历史未决责任，不能用空Outbox替代事务owner ready。

关闭先封新准入，等待已接受业务、NORMAL和订阅的真实在途，再排空本实例SDK调用，最后释放所属客户端及宿主资源。超时保留`ShutdownIncomplete`和未完成责任，允许继续等待；重复关闭不提前关闭仍被使用的数据库池。原真实gRPC handler与已进入的本地提交不因超时被伪称完成。

## 4. 当前维护写入

产生授权版本变化的维护操作使用`--outbox-mode transaction --transaction-messaging-config <受限JSON路径>`。该名称保留既有CLI参数，值只接受最终transaction；空值、legacy、standard不会构造旧发布stager。配置必须显式RocketMQ、单owner、原数据库绑定及准入；宿主持有root和生命周期。

[共享维护装配](../../cmd/iam-maintenance/transaction_eventing.go)启动当前owner，借用[policywrite](../../internal/apiserver/maintenance/policywrite/write.go)规划/提交原业务事务，保留指纹、停写、补偿及私有结果合同；未知结果或关闭未完成不能盲重跑命令。只读状态/preflight不制造通知。首次空库bootstrap仍承担独立初始化职责，不能等同于今天存量库的消息写入或删掉历史迁移。

## 5. 退役范围和保护资产

`m3-messaging`、`m3-messaging-recovery`、`reliable-messaging`旧切换CLI在解析外部配置或访问资源前明确拒绝；原离线安装/恢复实现、无生产调用的Relay与PolicyWirePublisher、M6一次性Go探针已退出当前源码。旧维护工具的历史成功报告不代表当前源支持重做M3切换。当前事务诊断/原生告警与具名处置由infra维护，不能自动改provider、重建原ID或重跑业务。

历史迁移文件、已发布SDK/tag、原数据/表/卷/备份不删除。身份到Session撤销、当前operation/receipt、first-wire、witness、Checker和恢复责任继续保护。数据资产的期限及处置结果由M4 C07登记；源码清理不产生DDL或数据清空。

## 6. 验证和生产口径

当前owner、真实原根、关闭与IAM双实例/QS独立订阅由infra固定候选真实Broker/MySQL门禁验证。源码CI另执行真实MySQL schema及维护业务事务、并发CAS、配置拒绝、race、文档和镜像检查。受控回调、实际SDK线程、真实进程故障和合法system业务验收分别记账。

[事务生命周期测试](../../internal/apiserver/container/platform/transaction_lifecycle_test.go)、[进程关闭测试](../../internal/apiserver/process/reliable_shutdown_test.go)与[维护拒绝测试](../../cmd/iam-maintenance/retired_messaging_test.go)保护当前入口。M4最终镜像、部署及业务验收由[infra总表](https://github.com/FangcunMount/infra/blob/main/docs/server/rocketmq-m4-execution.md)维护；源码合并不等于已部署，CD暂停也不等于业务停写。
