# IAM 文档中心

## 本文档中心定位

`docs/` 是 IAM 的解释层，负责说明系统边界、运行链路、设计取舍、业务模型、接入方式和源码阅读路径。

它不是机器契约本身：

- REST 路径、字段、schema 以 [`../api/rest`](../api/rest) 为准。
- gRPC service、message、RPC 以 [`../api/grpc`](../api/grpc) 为准。
- SDK 公开 API 以 [`../pkg/sdk`](../pkg/sdk) 为准。
- 运行行为以源码和测试为准。
- `_archive/` 只保存历史材料，不作为当前事实源。

---

## 30 秒结论

IAM 是一个面向业务系统接入的身份与访问管理服务。文档第一版重建的目标，是让读者按下面顺序建立心智模型：

```text
系统总览
  -> 运行时装配
  -> AuthN 认证链路
  -> AuthZ 授权链路
  -> Identity / ProfileLink 身份关系
  -> REST / gRPC / SDK 接入
  -> 架构护栏与防漂移机制
```

第一版文档不追求覆盖每一个源码文件，而是优先讲清楚：

- IAM 是什么，不是什么；
- 服务如何从 `cmd/apiserver` 启动到 REST/gRPC；
- `process / container / transport / application / domain / infra` 各层边界；
- AuthN 如何把登录变成 session/token/JWKS；
- AuthZ 如何把角色、资源、权限、RoleBinding 变成授权判定；
- ProfileLink 如何表达用户与业务档案关系；
- Outbox 如何传播授权版本变化；
- 架构测试如何防止边界回退；
- 后续应该如何读源码。

---

## 当前阅读入口

> 说明：文档体系正在按第一版重建计划逐步替换。已经重建的文档会使用链接；待重建文档先以路径形式列出，避免发布过程中出现断链。

| 你想回答的问题 | 推荐阅读 |
|---|---|
| IAM 是什么，整体架构怎么分层 | [00-概览/01-系统架构总览.md](00-概览/01-系统架构总览.md) |
| 服务如何从入口启动，生命周期如何装配 | `01-运行时/01-服务入口与生命周期装配.md` |
| REST/gRPC 如何从 container 获取模块能力 | `01-运行时/02-Transport装配--REST路由与gRPC服务注册.md` |
| 配置、运行模式、degraded startup 怎么理解 | `01-运行时/03-配置与运行模式.md`、`01-运行时/05-降级启动与健康检查.md` |
| 后台任务和 graceful shutdown 如何运行 | `01-运行时/04-后台任务与优雅关闭.md` |
| 登录如何变成 session、access token、refresh token | `02-认证AuthN/01-登录链路--从Login请求到Session与Token.md` |
| Token、Session、用户状态边界是什么 | `02-认证AuthN/02-认证语义--用户状态&会话&Token边界.md` |
| JWKS、key rotation、离线验签怎么工作 | `02-认证AuthN/03-JWKS与KeyRotation.md` |
| 微信/企微登录和 IDP 如何协作 | `02-认证AuthN/04-第三方登录与IDP协作.md` |
| 授权模型如何组织 | `03-授权AuthZ/01-授权模型--Role&Resource&Permission&RoleBinding.md` |
| 一次授权判定如何走到 Casbin | `03-授权AuthZ/02-授权判定链路--从Check到Casbin.md` |
| 授权写入为什么不是简单 CRUD | `03-授权AuthZ/03-PolicyChangeCommitter与UoW.md` |
| 授权版本事件和 Outbox 如何传播 | `03-授权AuthZ/04-授权版本事件与Outbox.md` |
| User、Profile、ProfileLink 如何建模 | `04-身份Identity/01-User与Profile模型.md`、`04-身份Identity/02-ProfileLink链路--用户与儿童档案关系协作.md` |
| REST/gRPC/SDK 如何接入 | `05-接入与契约/01-REST API契约.md`、`05-接入与契约/02-gRPC API契约.md`、`05-接入与契约/03-SDK接入模型.md` |
| 架构边界如何被测试保护 | `06-架构护栏/01-架构测试与依赖边界.md` |
| 文档事实源和防漂移机制是什么 | `06-架构护栏/02-文档事实源与防漂移机制.md` |

---

## 第一版文档体系

第一版重建完成后，文档目录建议组织为：

```text
docs/
├── README.md
├── 00-概览/
│   └── 01-系统架构总览.md
│
├── 01-运行时/
│   ├── 01-服务入口与生命周期装配.md
│   ├── 02-Transport装配--REST路由与gRPC服务注册.md
│   ├── 03-配置与运行模式.md
│   ├── 04-后台任务与优雅关闭.md
│   └── 05-降级启动与健康检查.md
│
├── 02-认证AuthN/
│   ├── 01-登录链路--从Login请求到Session与Token.md
│   ├── 02-认证语义--用户状态&会话&Token边界.md
│   ├── 03-JWKS与KeyRotation.md
│   └── 04-第三方登录与IDP协作.md
│
├── 03-授权AuthZ/
│   ├── 01-授权模型--Role&Resource&Permission&RoleBinding.md
│   ├── 02-授权判定链路--从Check到Casbin.md
│   ├── 03-PolicyChangeCommitter与UoW.md
│   └── 04-授权版本事件与Outbox.md
│
├── 04-身份Identity/
│   ├── 01-User与Profile模型.md
│   └── 02-ProfileLink链路--用户与儿童档案关系协作.md
│
├── 05-接入与契约/
│   ├── 01-REST API契约.md
│   ├── 02-gRPC API契约.md
│   └── 03-SDK接入模型.md
│
├── 06-架构护栏/
│   ├── 01-架构测试与依赖边界.md
│   └── 02-文档事实源与防漂移机制.md
│
└── _archive/
```

---

## 文档分层说明

### 00-概览

回答“这个系统是什么”。

重点说明：

- IAM 的系统定位；
- 运行时外部关系；
- 代码分层；
- 核心模块；
- 设计亮点；
- 推荐源码阅读路线。

### 01-运行时

回答“这个服务如何启动、装配和关闭”。

重点说明：

- `cmd/apiserver -> app -> config -> process.Run`；
- `PrepareRun()` stage pipeline；
- MySQL、Redis、IDP encryption key、EventBus 资源准备；
- container bootstrap；
- REST/gRPC transport registration；
- background tasks；
- graceful shutdown；
- degraded startup 与 health/debug 边界。

### 02-认证 AuthN

回答“用户如何登录，登录态如何被管理和验证”。

重点说明：

- 登录入口；
- SignInAdapterCatalog；
- AuthCredential proof；
- Authenticator/AuthStrategy；
- TokenIssuer；
- Session；
- Refresh Token；
- JWT；
- JWKS；
- KeyRotation；
- IDP / WeChat / WeCom 协作。

### 03-授权 AuthZ

回答“用户能不能访问某个资源”。

重点说明：

- Role；
- Resource；
- Permission；
- RoleBinding；
- assignment wire term 与 rolebinding internal term；
- AuthorizationRequest；
- AuthorizationDecision；
- Casbin facts；
- PolicyChange；
- PolicyChangeCommitter；
- Unit of Work；
- PolicyVersion；
- Transactional Outbox。

### 04-身份 Identity

回答“User、Profile、ProfileLink 如何表达业务身份关系”。

重点说明：

- User 是登录主体；
- Profile 是业务档案；
- ProfileLink 是 User 与 Profile 的关系；
- self profile/link 是基础不变量；
- MyProfileLinks 是当前用户视角的 guard；
- Suggest 只提供候选；
- AuthZ 才负责权限判定。

### 05-接入与契约

回答“外部系统如何接入 IAM”。

重点说明：

- REST 适合哪些调用方；
- gRPC 适合哪些调用方；
- SDK 如何隐藏底层细节；
- JWT 如何传递；
- JWKS 何时用于离线验签；
- 在线 Verify 何时必须使用；
- AuthZ Check 如何接入；
- public/admin/protected 能力边界。

### 06-架构护栏

回答“架构为什么不会轻易回退”。

重点说明：

- domain 不依赖 infra/database；
- application 不依赖 transport；
- REST router 不依赖 container/global config；
- assembler 使用 typed deps；
- AuthZ 内部统一 rolebinding；
- Casbin facts 不进入 domain/transport；
- docs 以源码/API 契约/测试为事实源；
- `_archive` 不作为当前事实源。

---

## 第一版重建步骤

第一版文档重建按 7 个 step 推进。

### Step 1：确定文档体系与发布边界

产出：

```text
docs/README.md
第一版文档清单
每篇文档回答的问题
每篇文档的事实源边界
```

完成标准：

```text
目录结构明确
待重建文档明确
_archive 边界明确
```

### Step 2：总览与运行时

产出：

```text
00-概览/01-系统架构总览.md
01-运行时/01-服务入口与生命周期装配.md
01-运行时/02-Transport装配--REST路由与gRPC服务注册.md
01-运行时/03-配置与运行模式.md
01-运行时/04-后台任务与优雅关闭.md
01-运行时/05-降级启动与健康检查.md
```

完成标准：

```text
读者能说清楚 IAM 如何从 main 启动到 REST/gRPC
读者能说清楚 process/container/transport 的边界
读者能顺着源码阅读入口、配置、stage、container、router、shutdown
```

### Step 3：认证 AuthN

产出：

```text
02-认证AuthN/01-登录链路--从Login请求到Session与Token.md
02-认证AuthN/02-认证语义--用户状态&会话&Token边界.md
02-认证AuthN/03-JWKS与KeyRotation.md
02-认证AuthN/04-第三方登录与IDP协作.md
```

完成标准：

```text
读者能讲清一次登录请求如何变成 session + access token + refresh token
读者能区分在线 Verify 与离线 JWKS 验签
读者能说清 IDP 与 AuthN 的边界
读者能解释 Adapter Catalog 和 Strategy 的设计价值
```

### Step 4：授权 AuthZ

产出：

```text
03-授权AuthZ/01-授权模型--Role&Resource&Permission&RoleBinding.md
03-授权AuthZ/02-授权判定链路--从Check到Casbin.md
03-授权AuthZ/03-PolicyChangeCommitter与UoW.md
03-授权AuthZ/04-授权版本事件与Outbox.md
```

完成标准：

```text
读者能区分 assignment 和 rolebinding
读者能说清 Casbin 在 IAM 中只是 infra adapter
读者能解释授权写入为什么不是简单 CRUD
读者能理解 PolicyChangeCommitter + UoW + Outbox 的设计价值
```

### Step 5：身份 Identity

产出：

```text
04-身份Identity/01-User与Profile模型.md
04-身份Identity/02-ProfileLink链路--用户与儿童档案关系协作.md
```

完成标准：

```text
读者能区分 User、Profile、ProfileLink
读者能解释为什么 ProfileLink 不能只是 User 的字段
读者能说清 self profile/link 的意义
读者能区分 ProfileLink、Suggest、AuthZ 的边界
```

### Step 6：接入与契约

产出：

```text
05-接入与契约/01-REST API契约.md
05-接入与契约/02-gRPC API契约.md
05-接入与契约/03-SDK接入模型.md
```

完成标准：

```text
读者知道 REST/gRPC/SDK 的接入边界
读者知道 JWT/JWKS/Verify/AuthZ Check 如何选择
读者能从文档跳转到 api/rest、api/grpc、pkg/sdk
```

### Step 7：架构护栏与发布检查

产出：

```text
06-架构护栏/01-架构测试与依赖边界.md
06-架构护栏/02-文档事实源与防漂移机制.md
docs/README.md 最终更新
发布检查清单
```

完成标准：

```text
文档体系有唯一入口
源码事实源清楚
旧文档不会误导读者
第一版可以发布
```

---

## 事实源优先级

文档中出现事实冲突时，按以下优先级判断：

1. **源码与运行时行为**  
   `cmd/`、`internal/apiserver/`、`internal/pkg/`、`pkg/`。

2. **机器契约、配置和迁移**  
   `api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`。

3. **架构测试和契约测试**  
   重点包括 `internal/pkg/architecture`、REST/gRPC transport tests、SDK compile tests。

4. **当前维护文档**  
   本目录、[`../api/README.md`](../api/README.md)、[`../pkg/sdk/docs/README.md`](../pkg/sdk/docs/README.md)。

5. **历史文档与归档材料**  
   `_archive/` 只用于历史追溯，不作为当前事实源。

---

## 术语约定

| 术语 | 当前约定 |
|---|---|
| IAM | Identity and Access Management 服务，不等同于单纯用户管理后台 |
| AuthN | Authentication，负责登录、账号、session、token、JWKS |
| AuthZ | Authorization，负责角色、资源、权限、RoleBinding、授权判定 |
| User | 登录主体 |
| Profile | 业务档案，例如儿童档案 |
| ProfileLink | User 与 Profile 的关系，可承载监护/亲属等业务语义 |
| assignment | 对外 REST/proto wire term，表示角色分配 |
| rolebinding | 内部 application/domain 标准术语，表示 subject 与 role 的绑定 |
| process | 生命周期编排层 |
| container | 组合根，不处理请求，不写业务规则 |
| transport | REST/gRPC 协议适配层 |
| application | 用例编排层 |
| domain | 领域规则层 |
| infra | MySQL、Redis、Casbin、JWT、Outbox、WeChat API 等外部资源适配层 |
| degraded startup | 非生产诊断/开发场景的受控降级启动，不是生产容错承诺 |
| transactional outbox | 业务事实和事件记录同事务提交，再由 relay 异步发布 |

---

## 每篇文档的建议结构

第一版文档尽量使用统一结构：

```text
# 标题

## 本文回答

## 30 秒结论

## 主图

## 重点速查

## 核心模型 / 核心链路

## 为什么这么设计

## 失败边界 / 安全边界 / 降级边界

## 设计模式

## 代码证据

## 推荐源码阅读路线

## 验证建议

## 本文总结
```

不是每篇都必须机械套满，但至少要包含：

```text
本文回答
30 秒结论
核心链路
代码证据
验证建议
```

---

## 维护规则

### 1. 不把文档写成源码转述

文档不是逐行解释代码。  
文档应该解释：

```text
为什么这样分层
为什么这样装配
为什么这样建模
失败边界在哪里
读者应该如何继续读源码
```

### 2. 不把 API 文档重复成字段清单

字段、路径、RPC 以 OpenAPI/proto 为准。  
`docs/` 只解释接入方式、语义边界和设计取舍。

### 3. 不从 `_archive` 复制当前事实

`_archive/` 可以用于历史追溯，但不能作为当前架构、当前代码、当前接口事实源。

### 4. 每篇文档必须能落到源码

文档中出现关键判断，应该能指向：

```text
源码路径
API 契约
配置文件
测试文件
```

### 5. 术语必须统一

尤其注意：

```text
ProfileLink 不要退回 Ref / GuardianRef 等旧术语
内部 AuthZ 不要退回 assignment 包
Casbin 不要被写成业务语言
container 不要被写成 service 层
process 不要被写成普通 router/server 包
```

---

## 发布检查清单

第一版文档发布前，至少检查：

```text
1. docs/README.md 能作为唯一入口
2. 每篇文档都有“本文回答”
3. 每篇文档都有“30 秒结论”
4. 每篇文档有源码入口或契约入口
5. Mermaid 图能解释核心链路
6. 待重建文档不以断链形式发布
7. _archive 不作为当前事实源
8. 业务术语统一
9. OpenAPI/proto/SDK 文档不互相矛盾
10. make docs-hygiene 无明显问题
```

---

## 验证命令

基础文档卫生检查：

```bash
make docs-hygiene
```

架构边界和运行时装配检查：

```bash
go test ./internal/pkg/architecture \
  ./internal/apiserver/process \
  ./internal/apiserver/container \
  ./internal/apiserver/transport/...
```

API 契约检查：

```bash
make api-validate
```

`make api-validate` 依赖 Docker daemon。Docker 不可用时，应至少单独运行仓库中的 OpenAPI、路由和契约检查脚本，并在提交说明中记录前置条件。
