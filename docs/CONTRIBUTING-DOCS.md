# IAM 文档写作约定

本文约定 IAM 活跃文档如何与代码、契约和历史材料对齐。读者入口见 [README.md](README.md)。

## 适用范围

- 适用于 `docs/` 下除 `_archive` 外的文档。
- 适用于 [../api/README.md](../api/README.md)、[../api/rest/README.md](../api/rest/README.md)、[../api/grpc/README.md](../api/grpc/README.md) 和 [../pkg/sdk/docs/README.md](../pkg/sdk/docs/README.md) 这类说明性入口。
- `docs/_archive` 是历史区，默认不适用活跃文档结构和卫生门禁。

## 事实来源与优先级

1. 源码与运行时行为：`cmd/`、`internal/apiserver/`、`pkg/`。
2. 机器契约与配置：`api/rest`、`api/grpc`、`configs`、`internal/pkg/migration/migrations`、`internal/apiserver/docs/swagger.yaml`。
3. 测试：架构测试、transport 契约测试、SDK compile tests。
4. 当前维护文档。
5. 历史文档和归档材料。

旧文档可以提供背景、术语和迁移线索，但进入活跃正文前必须重新核对代码或机器契约。

## 当前修订目标

| 问题 | 改法 |
| ---- | ---- |
| 重构后文档仍指向旧目录和旧路由 | 活跃文档统一到 `process + container + transport/rest|grpc`、`ProfileLink`、`rolebinding`、`application/authn/token`、`infra/token/*` |
| 根入口和 API 入口曾引用不存在的页面 | 根 README、docs README 和 API README 只链接当前存在的文件 |
| 历史文档和当前事实混在一起 | 历史材料进入 `_archive`，活跃文档不依赖归档事实 |
| 文档漂移缺少自动发现 | 使用 `make docs-hygiene` 检查链接和退役事实引用 |

## 术语规范

| 当前术语 | 说明 |
| ---- | ---- |
| `ProfileLink` | 用户与 profile 的当前代码/契约名；中文可解释为档案关系或监护关系语义 |
| `assignment` | REST/proto 的公开 wire term |
| `rolebinding` | AuthZ 内部应用和领域实现名 |
| `transport/rest`、`transport/grpc` | 当前协议适配层 |
| `transactional outbox` | 当前 durable event 发布路径 |
| `cache governance` | 只读缓存目录和状态治理面，不表示在线改写缓存 |

不要在活跃文档中把历史包路径、旧路由或未实现 RPC 写成当前事实。

## 文档结构

长文优先采用：

1. 本文回答。
2. 30 秒结论。
3. 当前事实表或链路图。
4. 详细展开。
5. 证据与回链。
6. 验证方式。

不是每篇文档都要机械套模板；索引页可以直接给阅读路径。

## 状态标签

- `已实现`：已经能从代码、契约或测试证明。
- `待补证据`：有实现迹象，但文档尚未给出足够证据。
- `规划改造`：尚未实现或需要另开任务。

使用状态标签时必须给出可核对的事实入口。

## 链接规则

- 活跃文档的相对链接必须解析到真实文件。
- 指向代码时优先链接当前源码目录。
- 指向 API 时优先链接 OpenAPI/proto 文件，而不是复制字段表。
- 归档文档可被根入口列出，但活跃正文不应依赖归档材料才能成立。
- 文件名含空格时使用 `%20` 或确保 Markdown 链接能被 `make docs-hygiene` 解析。

## 必跑验证

```bash
make docs-hygiene
```

涉及契约、路由、swagger 或 proto 时再运行：

```bash
make api-validate
go test ./internal/apiserver/transport/rest ./internal/apiserver/transport/grpc ./pkg/sdk
```

`make api-validate` 需要 Docker daemon；Docker 不可用时，在提交说明中记录原因，并尽量单独运行 [../scripts/check-openapi-contracts.py](../scripts/check-openapi-contracts.py) 与 [../scripts/check-route-contracts.py](../scripts/check-route-contracts.py)。

## 归档策略

- `_archive` 只保存历史上下文、旧分析和迁移参考。
- `_archive` 不参与默认 docs hygiene。
- 从 `_archive` 迁回活跃正文前，必须重新核对代码和契约。
- 历史报告可以保留结论，但不能作为当前实现的权威引用。
