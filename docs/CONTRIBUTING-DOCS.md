# IAM 文档维护约定

> 状态：已实现 · 本约定定义当前文档体系的维护、证据与验证规则。

## 1. 目录所有权

| 目录 | Canonical 内容 |
| --- | --- |
| `00-概览` | 系统地图、术语、阅读入口 |
| `01-运行时` | process/container/transport/lifecycle |
| `02-业务模块` | 各模块模型、用例、一致性和边界 |
| `03-基础设施` | MySQL、Redis、Event、Crypto、Transport、Observability |
| `04-接口与SDK` | REST、gRPC、Go SDK、接入与契约 |
| `05-工程质量与运维` | architecture/test/migration/release/security/docs governance |
| `06-专题设计` | 跨模块推理与面试高频专题 |
| `07-面试索引` | 只组织项目介绍、案例、追问链接和可公开贡献口径，不拥有技术事实 |
| `_archive` | 仅历史追溯，不是当前事实源 |

同一事实只在一个 canonical 文档讲透，其他位置用链接。面试索引必须回链 canonical 文档，不得成为第二事实源；不要为目录对称制造空文档，也不要另建数百行“宣讲版”复制正文。

## 2. 事实与状态

优先级：当前代码/runtime > machine contract/config/migration > tests > active docs > archive。

正文明确区分：

- 当前事实：代码今天真实执行；
- 设计决策：已采用方案及约束；
- 当前限制：已知失败窗口或未覆盖能力；
- 设计建议：尚未实现，不能作为能力承诺；
- 运行证据：带环境、版本、时间窗口；
- 验收台账：引用证据，不替代架构正文。

无法从代码/决策记录确认“为什么”时，应写成基于约束的分析或待确认，不编造初始意图。

## 3. 深度要求

设计文档至少回答：问题、责任、模型/链路、关键不变量、事务/失败语义、选择理由、替代方案、当前限制、事实来源和验证。面试问答放在相应 canonical 文档末尾。

不复制 OpenAPI/proto/SDK 全量字段表；只解释语义、边界和变更治理。

## 4. 路径与链接

- 使用真实 repo path，不写候选/伪路径；
- 可执行命令使用 `go test` 或仓库 Make target，不写个人 Go 安装路径；
- 文件重命名时同步所有入口和脚本引用；
- active docs 不依赖 archive 才能成立；
- 删除旧稿优先使用 Git 历史恢复，不保留无价值 redirect stub。

## 5. 变更同步

| 变更 | 至少检查 |
| --- | --- |
| REST | OpenAPI、Router、DTO、SDK/caller、route tests |
| gRPC | proto、generated、registration、service、SDK compile |
| domain/use case | module docs、事务/失败语义、focused tests |
| cache/event | truth source、TTL/version、metrics/readiness/shutdown |
| config/runtime | dev/prod config、RuntimeProfile、deployment probes |
| migration | immutable history、MySQL integration、rollout/rollback |
| public SDK | README/examples、compile test、migration note |

## 6. 最小验证

```bash
make docs-hygiene
make docs-facts
```

涉及对应事实时，再运行 architecture、contract、SDK、模块、MySQL 和 staging/production 验证。每类结果分别报告；未执行项写明原因。

## 7. Review 清单

- [ ] canonical owner 唯一且入口可达；
- [ ] 现状、建议和运行证据未混写；
- [ ] 设计理由与替代方案有真实约束；
- [ ] 失败、并发、降级和安全方向已说明；
- [ ] 代码/配置/contract/test path 存在；
- [ ] 没有敏感值、过期路径或历史术语回流；
- [ ] 删除旧稿后链接和事实脚本已更新；
- [ ] docs hygiene/facts 通过。
