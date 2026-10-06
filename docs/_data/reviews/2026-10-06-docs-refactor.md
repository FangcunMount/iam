# IAM文档逐篇深化：2026-10-06阶段发布

> 状态：历史记录 · 本批以c8fc1fe61a32056368148e232e96d32763c15ba5为源码基线，发布18篇完成复核的正文及必要支持文件。剩余重构继续在独立工作区进行。

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

发布工作区docs-hygiene检查106个现行/归档索引文件通过；docs-facts的机器契约、路由、配置及已编码事实通过，当前库存91篇已实现/1篇历史记录。三个历史分类回归通过；另外严格核对本批33个Markdown的文件与章节锚点，18篇正文与复核源文件字节一致，差异检查通过。文档门禁仅证明实际覆盖的引用与事实，不证明全文语义或环境行为。GitHub CI及远端main结果在完成后单独报告。

## 4. 剩余逐篇工作

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
