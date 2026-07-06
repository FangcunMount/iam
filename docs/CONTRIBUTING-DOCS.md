# IAM 文档写作约定

本文约定 `docs/` 下的现行文档写什么、写到什么粒度、如何与代码和契约对齐。读者入口见 [README.md](README.md)。

---

## 1. 适用范围

- 本文适用于 `docs/*` 下的现行文档。
- `docs/_archive/` 默认不适用现行结构要求。
- 历史文档只能作为信息源或迁移参考，不能直接视为现行事实。
- 迁移旧文档内容前，必须重新核对当前代码、OpenAPI、proto、配置、迁移或测试。

---

## 2. 当前重建目标

本轮文档重建的目标不是修补旧目录，而是建立一套以业务模块为核心的现行文档体系。

核心结构：

```text
docs/
├── 00-概览/
├── 01-运行时/
├── 02-业务模块/
│   ├── 01-Identity/
│   ├── 02-AuthN/
│   ├── 03-AuthZ/
│   ├── 04-IDP/
│   └── 05-Suggest/
├── 03-接入与契约/
├── 04-架构护栏/
├── 05-专题设计/
├── 06-宣讲/
└── _archive/
```

模块分层：

```text
核心模块：Identity、AuthN、AuthZ。
辅助模块：IDP、Suggest。
```

重建原则：

```text
模型优先；
核心优先；
事实优先；
链路随后；
设计取舍再后；
宣讲最后。
```

---

## 3. 事实来源与优先级

当文档、代码、契约或历史材料冲突时，按下面顺序判断：

1. 源码与运行时行为。
2. 机器可读契约与配置：OpenAPI、proto、配置、迁移。
3. 测试：架构测试、契约测试、模块测试、SDK compile test。
4. 现行维护中的 `docs/`。
5. `_archive/` 历史材料。

规则：

- 代码变了，文档跟着改。
- 契约变了，接入文档跟着改。
- 架构边界变了，架构护栏和宣讲材料跟着改。
- `_archive/` 只用于追溯和迁移参考，不能作为当前事实源。
- 旧文档中的事实进入现行正文前，必须重新核对代码、OpenAPI、proto、配置、迁移或测试。

---

## 4. 文档分层边界

| 目录 | 只写什么 | 不写什么 |
| --- | --- | --- |
| `00-概览/` | IAM 定位、模块关系、核心术语、阅读路径、事实源优先级 | 不展开具体模块实现细节 |
| `01-运行时/` | 服务入口、生命周期、组合根、REST/gRPC 装配、配置、后台任务、健康检查 | 不展开业务领域模型 |
| `02-业务模块/` | Identity、AuthN、AuthZ、IDP、Suggest 的当前模型、链路、边界、代码索引 | 不写长篇设计取舍和面试讲法 |
| `03-接入与契约/` | REST、gRPC、Go SDK、业务系统接入、契约事实源 | 不复制完整 OpenAPI/proto 字段清单 |
| `04-架构护栏/` | 分层依赖、架构测试、契约测试、SDK compile test、docs hygiene | 不写业务链路细节 |
| `05-专题设计/` | 为什么这样设计、替代方案、边界取舍、讲解口径 | 不替代业务模块事实层 |
| `06-宣讲/` | 面试、技术分享、图素材、追问证据链 | 不创造事实，只回链事实层和专题层 |
| `_archive/` | 历史材料、迁移参考 | 不作为当前事实源 |

---

## 5. 表达习惯

- 先结论，再展开。
- 先领域模型，再关键链路。
- 图、表、步骤优先于长段混合描述。
- 一处讲透，其他文档回链。
- 现状、证据、规划要分层，不把规划写成已实现事实。
- 业务模块文档写“当前事实”；专题设计文档写“为什么这样设计”。
- 宣讲文档必须能回链到业务模块、专题设计、源码、契约或测试。

---

## 6. 状态标签

当代码和文档成熟度不一致时，使用统一标签：

| 标签 | 含义 |
| --- | --- |
| `已实现` | 已有代码、契约或测试证据支撑 |
| `待补证据` | 文档方向可能正确，但还没有完成代码事实核对 |
| `规划改造` | 属于后续演进方向，不能写成当前已实现事实 |

使用要求：

- 占位文件默认标记为 `待补证据`。
- 从 `_archive/` 迁移来的内容默认标记为 `待补证据`，直到重新核对当前代码。
- 未落地能力必须标记为 `规划改造`。
- 已落地能力要在 `代码事实源` 中列出源码、契约或测试路径。

---

## 7. 长文固定结构

除纯索引页外，长文优先采用：

```text
1. 本文回答
2. 30 秒结论
3. 主图或主表
4. 详细展开
5. 关键边界
6. 代码事实源
7. Verify
8. 本文总结
```

推荐模板：

```markdown
# 文档标题

## 1. 本文回答

本文回答：

- ...
- ...

## 2. 30 秒结论

...

## 3. 主图 / 主表

```mermaid
flowchart TD
    A --> B
```

## 4. 详细展开

...

## 5. 关键边界

| 边界 | 说明 |
| --- | --- |
| ... | ... |

## 6. 代码事实源

| 事实 | 路径 |
| --- | --- |
| ... | `...` |

## 7. Verify

```bash
make docs-hygiene
```

## 8. 本文总结

...

---

## 8. 业务模块文档结构

每个业务模块优先使用：

```text
README.md
00-模块总览.md
01-领域模型-xxx.md
02-领域模型图.md
03-核心对象生命周期.md
04-关键链路-xxx.md
05-关键链路-xxx.md
06-模块边界-xxx.md
07-分层架构与代码索引.md
```

核心模块可以增加关键链路文档；辅助模块保持轻量。

业务模块文档固定顺序：

```text
模块定位
  -> 领域模型
  -> 领域模型图
  -> 核心对象生命周期
  -> 关键链路
  -> 模块边界
  -> 分层架构与代码索引
```

业务模块文档只讲当前事实：

```text
当前模型是什么；
当前对象如何变化；
当前链路怎么走；
当前边界是什么；
当前代码在哪里。
```

不在业务模块文档里长篇讨论替代方案。替代方案和设计取舍放入 `05-专题设计/`。

---

## 9. 业务模块重建顺序

### 9.1 第一批：文档骨架验收

先确保所有目录、README、链接和占位文件可用。

重点文件：

```text
docs/README.md
docs/CONTRIBUTING-DOCS.md
docs/00-概览/README.md
docs/02-业务模块/README.md
docs/02-业务模块/00-模块协作总图.md
```

验收标准：

```text
make docs-hygiene 通过；
README 中没有旧路径；
active docs 不用 _archive 证明当前事实；
所有占位文件都能从 README 链接到；
所有占位文件标记为待补证据。
```

### 9.2 第二批：Identity 模型闭环

Identity 是共同身份事实来源，优先重建。

重点文件：

```text
docs/02-业务模块/01-Identity/00-模块总览.md
docs/02-业务模块/01-Identity/01-领域模型-User-Profile-ProfileLink.md
docs/02-业务模块/01-Identity/02-领域模型图.md
docs/02-业务模块/01-Identity/03-核心对象生命周期.md
```

验收标准：

```text
能解释 User / Profile / ProfileLink；
能解释 ProfileLink 为什么不是 Permission；
能解释 Principal / Subject 为什么不是 User；
能回链代码事实源。
```

### 9.3 第三批：AuthN 模型闭环

AuthN 负责认证域，第二优先级重建。

重点文件：

```text
docs/02-业务模块/02-AuthN/00-模块总览.md
docs/02-业务模块/02-AuthN/01-领域模型-LoginIdentity-Credential-Challenge-Principal-Session-Token.md
docs/02-业务模块/02-AuthN/02-领域模型图.md
docs/02-业务模块/02-AuthN/03-核心对象生命周期.md
```

验收标准：

```text
能解释 LoginIdentity / Credential / Challenge；
能解释 Principal / Session / AccessToken / RefreshToken；
能解释 AuthN 和 IDP 的边界；
能说明当前 Credential 已实现类型，不把规划写成事实。
```

### 9.4 第四批：AuthZ 模型闭环

AuthZ 负责授权域，第三优先级重建。

重点文件：

```text
docs/02-业务模块/03-AuthZ/00-模块总览.md
docs/02-业务模块/03-AuthZ/01-领域模型-Subject-Resource-Action-Scope-Role-Permission-RoleBinding.md
docs/02-业务模块/03-AuthZ/02-领域模型图.md
docs/02-业务模块/03-AuthZ/03-核心对象生命周期.md
```

验收标准：

```text
能解释授权判定句式；
能解释 Role / Permission / RoleBinding；
能解释 Assignment 只是 wire term；
能解释 Casbin 不是领域模型；
能回链代码事实源。
```

### 9.5 第五批：三大核心链路

模型稳定后，再写关键链路。

Identity：

```text
04-关键链路-创建User与Profile.md
05-关键链路-建立与撤销ProfileLink.md
```

AuthN：

```text
04-关键链路-Onboarding身份开通.md
05-关键链路-Linking登录身份绑定.md
06-关键链路-Login登录认证.md
07-关键链路-Token签发刷新吊销.md
08-关键链路-JWKS与本地验签.md
```

AuthZ：

```text
04-关键链路-权限检查Check.md
05-关键链路-授权写入Grant-Revoke-Bind-Unbind.md
06-关键链路-授权版本传播PolicyVersion-Outbox-RuntimeReload.md
07-Casbin运行时模型.md
```

验收标准：

```text
每篇有主流程图；
每篇有对象状态变化；
每篇有失败边界；
每篇有代码事实源；
每篇有 Verify。
```

### 9.6 第六批：辅助模块

核心模块稳定后，再补 IDP 和 Suggest。

IDP 优先文件：

```text
docs/02-业务模块/04-IDP/00-模块总览.md
docs/02-业务模块/04-IDP/01-领域模型-WechatApp-Credentials-AppToken-ExternalIdentity.md
docs/02-业务模块/04-IDP/02-领域模型图.md
docs/02-业务模块/04-IDP/05-关键链路-外部身份解析与AuthN协作.md
```

Suggest 优先文件：

```text
docs/02-业务模块/05-Suggest/00-模块总览.md
docs/02-业务模块/05-Suggest/01-领域模型-ProfileSearchTerm-ProfileAccessScope-Snapshot.md
docs/02-业务模块/05-Suggest/02-领域模型图.md
docs/02-业务模块/05-Suggest/03-关键链路-SuggestProfile查询.md
docs/02-业务模块/05-Suggest/04-关键链路-索引刷新Full-Delta-Snapshot.md
```

### 9.7 第七批：专题设计

业务模块事实层稳定后，再写设计取舍。

建议顺序：

```text
docs/05-专题设计/02-Session-AccessToken-RefreshToken边界.md
docs/05-专题设计/01-JWT-JWS-JWK-JWKS-KeyRotation.md
docs/05-专题设计/04-Casbin在AuthZ中的定位.md
docs/05-专题设计/03-Transactional-Outbox设计.md
docs/05-专题设计/05-ProfileLink为什么不是Permission.md
docs/05-专题设计/06-Suggest为什么是读模型.md
```

专题设计写作重点：

```text
为什么需要这个设计；
不这样设计会有什么问题；
当前实现怎么做；
替代方案与取舍；
和业务模块事实层的关系；
代码事实源和 Verify。
```

### 9.8 第八批：宣讲材料

宣讲材料最后写。宣讲不创造事实，只组织表达。

宣讲材料必须回链：

```text
业务模块事实层；
专题设计；
源码；
契约；
测试。
```

---

## 10. 模块边界写作规则

### 10.1 Identity 边界

Identity 负责：

```text
User；
Profile；
ProfileLink；
身份事实查询。
```

Identity 不负责：

```text
Credential；
Challenge；
Principal；
Session；
Token；
Subject；
Permission；
RoleBinding；
Suggest 索引刷新和查询。
```

### 10.2 AuthN 边界

AuthN 负责：

```text
LoginIdentity；
Credential；
Challenge；
Principal；
Session；
Token；
JWKS。
```

AuthN 不负责：

```text
Role；
Permission；
RoleBinding；
ProfileLink；
Casbin Runtime；
Suggest 索引；
外部身份源配置所有权。
```

### 10.3 AuthZ 边界

AuthZ 负责：

```text
Subject；
Resource；
Action；
Scope；
Role；
Permission；
RoleBinding；
AuthorizationDecision；
PolicyVersion；
授权版本传播。
```

AuthZ 不负责：

```text
登录认证；
Token 签发；
User/Profile 写模型；
ProfileLink 关系治理。
```

### 10.4 IDP 边界

IDP 负责：

```text
外部身份源适配；
外部应用配置；
外部密钥治理；
外部 access token 获取与缓存；
外部身份声明解析。
```

IDP 不负责：

```text
创建 IAM 登录态；
签发 IAM Token；
拥有 User；
决定权限。
```

### 10.5 Suggest 边界

Suggest 负责：

```text
Profile 联想搜索读模型；
ProfileSearchTerm；
ProfileAccessScope；
Snapshot；
索引刷新；
手机号搜索安全策略。
```

Suggest 不负责：

```text
Profile 写模型；
登录认证；
通用授权策略管理；
User/Profile/ProfileLink 的主事实维护。
```

---

## 11. 术语规则

必须统一使用：

```text
User / Profile / ProfileLink；
LoginIdentity / Credential / Challenge / Principal / Session / AccessToken / RefreshToken；
Subject / Resource / Action / Scope / Role / Permission / RoleBinding / AuthorizationDecision；
WechatApp / Credentials / AppToken / ExternalIdentity；
ProfileSearchTerm / ProfileAccessScope / Snapshot；
Assignment 仅作为 REST/proto/SDK 对外 wire term；
Casbin 是 infra runtime，不是领域模型；
Outbox 是事务内事件记录和异步发布机制，不等于 exactly-once。
```

禁止把这些旧说法写成当前事实：

```text
Account 作为当前 AuthN 核心模型；
ProfileLink 写成 Permission；
Subject 写成 User 本体；
Principal 写成 User 本体；
IDP 写成登录态所有者；
Suggest 写成核心身份域；
Casbin 写成业务领域模型；
_archive 写成当前事实源。
```

---

## 12. 链接规则

- 现行文档之间使用相对链接。
- 不从现行正文回链 `_archive/` 来证明当前事实。
- 可在 archive README 中说明历史快照位置。
- 改目录或文件名后必须运行 `make docs-hygiene`。
- 业务模块文档之间应优先回链 `02-业务模块/00-模块协作总图.md` 和对应模块 README。
- 专题设计文档应回链对应业务模块事实层。
- 宣讲文档应回链业务模块、专题设计、源码、契约或测试。

---

## 13. Verify 规则

提交前至少运行：

```bash
make docs-hygiene
```

涉及架构边界时再运行：

```bash
go test ./internal/pkg/architecture
```

涉及 REST 契约时再运行：

```bash
make api-validate
```

涉及 gRPC 契约时再运行：

```bash
make proto-gen
go test ./internal/apiserver/transport/grpc
```

涉及 SDK 公开 API 时再运行：

```bash
go test ./pkg/sdk/...
```

涉及具体业务模块时，按模块补充对应测试命令。没有测试命令时，在 `Verify` 中写明“待补模块级测试命令”，不能假装已有。

---

## 14. 维护节奏

以下变更必须同步文档：

- 业务模块边界变化。
- OpenAPI、proto、SDK 公开 API 变化。
- 运行时入口、组合根、配置、后台任务变化。
- 架构测试或契约测试保护范围变化。
- 将 archive 内容迁入 active docs。
- 新增核心对象、关键状态、关键链路或跨模块依赖。

---

## 15. 对旧文档体系的处理

- 旧文档体系是信息源，不是权威来源。
- 迁移旧内容前必须核对当前代码。
- 旧结构可参考，但不保留为 active docs 的组织方式。
- 迁移不进去的内容进入 `_archive/`，不要强行塞入现行正文。
- 从 `_archive/` 迁入的内容，必须先标记为 `待补证据`，核对完成后才能改为 `已实现`。