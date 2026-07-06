# IAM 文档中心

## 30 秒结论

IAM 是面向业务系统接入的身份与访问管理服务。它围绕三个核心问题组织能力：

```text
用户是谁？        -> Identity
如何证明用户身份？ -> AuthN + IDP
用户能访问什么？   -> AuthZ
```

`docs/` 是解释层，不是事实本身。当前文档以 `02-业务模块/` 为核心事实层：先讲领域模型，再讲关键链路，最后回到模块边界和代码索引。

## 事实源优先级

当文档、代码、契约或历史材料冲突时，按下面顺序判断：

1. 源码与运行时行为
2. 机器可读契约与配置：OpenAPI、proto、配置、迁移
3. 测试：架构测试、契约测试、模块测试、SDK compile test
4. 现行 `docs/` 文档
5. `_archive/` 历史材料

`_archive/` 只用于追溯和迁移参考，不能作为当前事实源。

## 当前目录

```text
docs/
├── README.md
├── CONTRIBUTING-DOCS.md
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

| 目录 | 作用 |
| --- | --- |
| [00-概览](00-概览/README.md) | IAM 定位、模块关系、术语、阅读路径、事实源 |
| [01-运行时](01-运行时/README.md) | 服务入口、生命周期、组合根、REST/gRPC 装配、配置和后台任务 |
| [02-业务模块](02-业务模块/README.md) | Identity、AuthN、AuthZ、IDP、Suggest 的当前事实层 |
| [03-接入与契约](03-接入与契约/README.md) | REST、gRPC、Go SDK 和业务系统接入方式 |
| [04-架构护栏](04-架构护栏/README.md) | 分层依赖、架构测试、契约测试、SDK compile test、docs hygiene |
| [05-专题设计](05-专题设计/README.md) | JWT、Session/Token、Outbox、Casbin、ProfileLink、Suggest 的设计取舍 |
| [06-宣讲](06-宣讲/README.md) | 面试、技术分享、讲法脚本、图素材和追问证据链 |
| [_archive](_archive/README.md) | 历史文档和重构前快照，不作为当前事实源 |

## 推荐阅读路径

### 新读者

```text
00-概览/README.md
  -> 00-概览/01-IAM系统定位.md
  -> 00-概览/02-模块划分与协作关系.md
  -> 02-业务模块/README.md
```

目标：先建立 IAM 的边界、模块关系和核心术语。

### 后端开发

```text
00-概览/04-阅读路径与事实源优先级.md
  -> 01-运行时/README.md
  -> 01-运行时/02-组合根与依赖装配.md
  -> 02-业务模块/README.md
```

目标：先看服务如何装配，再进入模块模型和链路。

### 接入方

```text
03-接入与契约/README.md
  -> 03-接入与契约/01-REST接入契约.md
  -> 03-接入与契约/02-gRPC接入契约.md
  -> 03-接入与契约/03-Go-SDK接入模型.md
```

目标：明确接入形态、契约事实源和防漂移规则。

### 宣讲

```text
06-宣讲/README.md
  -> 05-专题设计/README.md
  -> 02-业务模块/00-模块协作总图.md
```

目标：先用稳定讲法建立主线，再回链事实层。

### 文档维护者

```text
CONTRIBUTING-DOCS.md
  -> 04-架构护栏/05-Docs-Hygiene.md
  -> make docs-hygiene
```

目标：避免旧目录、旧路由、旧术语和 archive 事实回流。

## 验证入口

最小验证：

```bash
make docs-hygiene
```

涉及架构边界：

```bash
go test ./internal/pkg/architecture
```

涉及 REST、gRPC、SDK 契约：

```bash
make api-validate
make proto-gen
go test ./internal/apiserver/transport/grpc
go test ./pkg/sdk/...
```
