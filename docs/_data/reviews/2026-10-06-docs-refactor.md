# IAM文档逐篇深化与阶段发布记录

> 状态：历史记录 · 记录2026-10-06首批18篇与2026-10-07第二批21篇的来源、范围及验证边界；剩余重构继续在独立工作区进行。

## 1. 本批正文

| 范围 | 完成数量 | 主文 |
| --- | ---: | --- |
| AuthN | 7 | [模型与策略](../../02-业务模块/02-AuthN/01-领域模型与认证策略.md)、[注册](../../02-业务模块/02-AuthN/02-注册登录与身份绑定.md)、[Session合同](../../02-业务模块/02-AuthN/03-Session-Token与JWKS.md)、[Linking](../../02-业务模块/02-AuthN/03-关键链路-Linking登录身份绑定.md)、[Login](../../02-业务模块/02-AuthN/04-关键链路-Login登录认证.md)、[Token](../../02-业务模块/02-AuthN/05-关键链路-Token签发刷新吊销.md)、[JWKS](../../02-业务模块/02-AuthN/06-关键链路-JWKS与本地验签.md) |
| AuthZ | 10 | [模型](../../02-业务模块/03-AuthZ/01-领域模型设计.md)、[判定](../../02-业务模块/03-AuthZ/02-关键链路-授权判定与不可变快照.md)、[写入](../../02-业务模块/03-AuthZ/03-关键链路-授权写入与受管Assignment.md)、[收敛](../../02-业务模块/03-AuthZ/04-关键链路-多实例策略收敛.md)、[REST](../../02-业务模块/03-AuthZ/05-关键链路-REST管理与路由授权.md)、[gRPC/SDK](../../02-业务模块/03-AuthZ/06-关键链路-gRPC服务间授权与SDK.md)、[协作](../../02-业务模块/03-AuthZ/07-模块边界-AuthZ与AuthN-Identity-Suggest.md)、[分层](../../02-业务模块/03-AuthZ/08-分层架构与代码索引.md)、[条件退役](../../02-业务模块/03-AuthZ/08-条件授权退役维护手册.md)、[安全验收](../../02-业务模块/03-AuthZ/09-安全加固与发布验收.md) |
| 专题 | 1 | [身份、认证与授权边界](../../06-专题设计/01-身份认证与授权边界.md) |

主文均追踪实际入口、事实源、事务/并发及失败窗口，用具体对象与时序解释当前收益和代价。内部调用合同、源码推演、未实现的候选约束及真实环境证据分别标明；没有用代码状态或测试通过宣告部署/业务验收。

Session模型篇纠正不存在的AccessTokenProjector、公开入口/证明kind/结果Method混用、所有令牌均秒级、schema_version等于实体版本和全部快照不可变的概括。用空typed上下文、旧JWT/当前Session身份、当前policy收紧/放宽、双写认证时间与历史长寿命对象，说明恢复/校验与退役证据的边界。

## 2. 支持同步与范围

本批同步AuthN/AuthZ导航、关联核心模型PNG/SVG以及五篇SDK说明中的接口、超时、验签与迁移事实。主文直接依赖的[账号开通](../../operations/account-provision.md)、[授权写入冻结](../../operations/authz-write-freeze.md)、[统计运营目录](../../operations/statistics-operations-catalog.md)作为支持手册带入；三者仍列在后续手册深化清单，不额外计入18篇。

[条件退役原稿](../../_archive/2026-10-06-docs-refactor/README.md)保留历史样本。本批未复制完整60篇快照或其他待深化正文；Identity边界及事件主文仅增加兼容HTML锚点，不计作整篇完成。未变更业务源码、业务配置、proto/OpenAPI或部署逻辑。检查器只增加本阶段历史记录分类、当前库存输出及核心图术语，CI增加三个分类回归。完整第一轮结构/事实校准、其他图源及全量新门禁继续保留在重构工作区，后续批次分别验收。

## 3. 本地验证与限制

每篇在重构过程中运行相关既有测试并核对其证明范围；源码基线保持相同。最新Session模型批15个Go package实际执行用例并通过race，没有Skip或仅编译包：上下文、Session/Token、SignIn与应用、Redis、RSA codec、authnclaims、requestctx、middleware、REST/gRPC、SDK策略和架构护栏。

```bash
go test -race -count=1 ./internal/apiserver/domain/authn/authentication \
  ./internal/apiserver/domain/authn/session ./internal/apiserver/domain/authn/token \
  ./internal/apiserver/application/authn/session ./internal/apiserver/application/authn/token \
  ./internal/apiserver/application/authn/signin ./internal/apiserver/infra/cache/redis \
  ./internal/apiserver/infra/token/jwt ./internal/pkg/authnclaims ./internal/pkg/requestctx \
  ./internal/pkg/middleware/authn ./internal/apiserver/transport/rest/authn/handler \
  ./internal/apiserver/transport/grpc/service/authn ./pkg/sdk/auth/verifier ./internal/pkg/architecture
```

本地使用miniredis、SQLite/端口替身、真实临时RSA；部分Session/Admission为内存/放行替身，传输一致性直接调用handler/service。此前JWKS批的19个包中17个实际测试通过、两包仅编译，18项真实MySQL专项Skip；这些结果不证明真实MySQL/Redis、mTLS路由、多实例、共享卷或业务消费者验收。各主文列出尚缺专项。

新Mermaid按篇真实离线渲染并查看。最新两张模型图为1092×1054、586×862，正文/图源SHA与报告一致、文字可读、无裁切/重叠；JWKS公钥来源与PEM分开，在线验证/刷新身份来源分开。发布复制保持18篇正文与已复核源文件字节一致。

发布工作区docs-hygiene检查106个现行/归档索引文件通过；docs-facts的机器契约、路由、配置及已编码事实通过，当前库存91篇已实现/1篇历史记录。三个历史分类回归通过；另外严格核对本批33个现行Markdown及归档索引的文件与章节锚点，保留原稿另按基线对照正文、不要求历史链接有效。18篇正文与复核源文件字节一致，差异检查通过。文档门禁仅证明实际覆盖的引用与事实，不证明全文语义或环境行为。GitHub CI及远端main结果在完成后单独报告。

## 4. 首批发布后的剩余逐篇工作（2026-10-06）

完整重构库存为65篇技术正文与操作手册，18篇完成、47篇待深化。README/模块总览、表达材料与历史记录另行统一核对，不加入完成比例；其他正文已有第一轮校准，仍需按篇补场景、取舍和边界。

| 范围 | 剩余数量 | 篇章 |
| --- | ---: | --- |
| AuthN | 2 | 模块边界；分层与代码索引 |
| Identity | 5 | 模型；创建User/Profile；ProfileLink；边界；分层 |
| IDP | 4 | 凭据/AppToken；外部解析；信任模型；边界 |
| Suggest | 4 | 模型/端口；Full-Delta；查询；边界 |
| 运行时 | 3 | 启动；配置/传输；后台任务/就绪/关闭 |
| 基础设施 | 6 | MySQL；Redis；Outbox；密码学；传输安全；观测 |
| 接口与SDK | 2 | 契约治理；业务接入 |
| 质量与运维 | 6 | 架构护栏；测试验收；迁移发布；日志/凭据；文档治理；退役 |
| 专题 | 5 | 事务一致性；威胁；JWT/JOSE；Suggest读模型；角色图演化 |
| 概览 | 5 | 定位；协作；术语；架构原则；统一模型 |
| 操作手册 | 5 | 账号开通；Scope迁移；写入冻结；reviewer；统计目录 |

下一篇是AuthN模块边界；本次阶段发布不结束整套文档重构目标。

## 5. 第二批：第十九至三十九篇（2026-10-07）

本批以`3e34b7488089ce76d1ade05da993a8c273cb9b6f`为源码基线，增加21篇已完成逐篇复核的正文；与首批18篇合计39篇，技术正文与操作手册总计65篇，剩余26篇继续在独立重构工作区深化。第40篇密码学尚未完成，不在本批正文中。

| 范围 | 本批数量 | 正文 |
| --- | ---: | --- |
| AuthN | 2 | [模块边界](../../02-业务模块/02-AuthN/07-模块边界-AuthN与Identity-IDP-AuthZ.md)、[分层与代码索引](../../02-业务模块/02-AuthN/08-分层架构与代码索引.md) |
| Identity | 5 | [模型](../../02-业务模块/01-Identity/01-领域模型-User-Profile-ProfileLink.md)、[创建](../../02-业务模块/01-Identity/02-关键链路-创建User与Profile.md)、[ProfileLink](../../02-业务模块/01-Identity/03-关键链路-建立与撤销ProfileLink.md)、[边界](../../02-业务模块/01-Identity/04-模块边界-Identity与AuthN-AuthZ-Suggest.md)、[分层](../../02-业务模块/01-Identity/05-分层架构与代码索引.md) |
| IDP | 4 | [凭据与AppToken](../../02-业务模块/04-IDP/01-应用凭据与AppToken缓存.md)、[外部身份解析](../../02-业务模块/04-IDP/02-外部身份解析与AuthN协作.md)、[信任模型](../../02-业务模块/04-IDP/03-外部身份信任模型与方案演化.md)、[边界与代码索引](../../02-业务模块/04-IDP/04-模块边界与代码索引.md) |
| Suggest | 4 | [模型与端口](../../02-业务模块/05-Suggest/01-模型与应用端口.md)、[刷新](../../02-业务模块/05-Suggest/02-关键链路-索引刷新Full-Delta.md)、[查询](../../02-业务模块/05-Suggest/03-关键链路-SuggestProfile查询.md)、[边界与代码索引](../../02-业务模块/05-Suggest/04-模块边界与代码索引.md) |
| 运行时 | 3 | [启动](../../01-运行时/01-启动与组合根.md)、[配置与传输](../../01-运行时/02-配置与传输装配.md)、[后台任务、就绪与关闭](../../01-运行时/03-后台任务就绪与优雅关闭.md) |
| 基础设施 | 3 | [MySQL](../../03-基础设施/01-MySQL事务与迁移.md)、[Redis](../../03-基础设施/02-Redis与缓存一致性.md)、[事件与Outbox](../../03-基础设施/03-事件与Transactional-Outbox.md) |

各篇说明入口、事实源、资源与事务责任、成功/失败窗口、具体情境及尚未实现的候选设计。21篇正文保持与逐篇复核稿字节一致；目录导航、已发布AuthN/AuthZ正文中的必要引用和一个dirty处置兼容锚点随附同步。Suggest查询隐私断言改为检查实际材料路径与日志边界，避免沿用“原始手机号仅存在索引、所有日志均不记录”的过强结论。完整第一轮改稿、其余26篇正文、全量图/归档和新门禁留在重构工作区，本批不复制。

逐篇验证沿用当前源码基线：最近事件/Outbox批14个包的既有离线回归共73项通过，包含8个整包与6个指定用例选择，无Skip；测试范围包括IAM适配/编排、锁定版本可靠消息SDK Relay和NSQ客户端替身。此前各篇另外使用miniredis、SQLite、临时RSA及端口替身；具体证明范围与缺失专项由相应主文维护。源码、单元测试与时序图均不证明真实MySQL lease/audit、NSQ崩溃交接、短信接受、生产部署或业务验收。

发布树使用`make docs-hygiene docs-facts`与三个历史分类回归；另从重构工作区的新版检查器只读核对发布树的文件链接和章节锚点，不将全量新门禁混入本批。GitHub检查、远端main与合并提交在实际完成后记录；这次发布范围为文档合并与推送，不包括生产部署、版本标签或软件Release。

| 剩余范围 | 数量 | 待逐篇复核 |
| --- | ---: | --- |
| 基础设施 | 3 | 密码学；传输安全；可观测性 |
| 接口与SDK | 2 | REST/gRPC契约治理；Go SDK与业务接入 |
| 工程质量与运维 | 6 | 架构护栏；测试/验收；迁移/发布；日志/凭据；文档治理；遗留资产退役 |
| 专题设计 | 5 | 事务/缓存/事件；威胁模型；JWT/JOSE；Suggest读模型；角色图演化 |
| 概览 | 5 | 定位；模块协作；术语；架构原则；统一模型 |
| operations手册 | 5 | 账号开通；Scope迁移；写入冻结；reviewer开通；统计操作目录 |

目录总览、导航、表达材料、图源与跨文引用随后统一核对；阶段发布不结束文档重构目标。
