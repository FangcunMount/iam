# IAM 重构最终验收记录

> 状态：已实现 · 本文准确记录仓库侧验收与仍需生产侧完成的证据；未完成项不视为已验收。

## 1. 当前结论

仓库侧 5.4.1 变更已经建立可执行门禁；最终生产验收仍未完成。缺少的生产证据必须由对应 workflow 或运维记录产生，不能根据当前系统状态反推，也不能为了补记录再次清除 Refresh Token 或历史日志。

## 2. 安全记录规则

本文只登记 SHA、镜像 digest、发布时间、workflow run URL/ID、数量、字节、时间、operator、ticket 和通过/失败结论。禁止粘贴 Token、SQL、日志正文、Redis key、数据库地址或凭据。

## 3. 仓库侧证据

| 项目 | 状态 | 证据入口 |
| --- | --- | --- |
| AuthZ `Check` / `GetAuthorizationSnapshot` 安全错误 | 已实现 | `internal/apiserver/transport/grpc/service/authz/service.go`、`service_test.go`、`internal/pkg/architecture/architecture_test.go` |
| 数据库操作单一脚本 | 已实现 | `scripts/dbops/database-operation.sh`、`database_operation_test.go` |
| MySQL 8 合成备份恢复 | 已实现门禁 | `.github/workflows/concurrency-tests.yml`；需在最终 SHA 的 workflow 成功后登记 run URL |
| 文档状态与占位语门禁 | 已实现 | `scripts/check-docs-facts.py`、`make docs-hygiene docs-facts` |
| Active docs 语义分类 | 已完成本轮分类 | 27 篇 `已实现`、60 篇 `规划改造`、0 篇旧 `设计目标`；规划文档不得作为现有能力承诺 |

## 4. 最终发布证据

以下项目均为最终验收必需项。当前未完成项具有明确责任方和完成条件，不代表已经通过。

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| 最终源码 SHA | 未完成 | 发布负责人 | 5.4.1-C 合并后登记完整 SHA |
| 镜像 digest 与发布时间 | 未完成 | 发布负责人 | 登记不可变 digest 与 RFC3339 发布时间 |
| CI | 未完成 | 发布负责人 | 登记同一最终 SHA 的成功 run URL |
| CodeQL | 未完成 | 发布负责人 | 登记同一最终 SHA 的成功 run URL |
| MySQL 8 workflow | 未完成 | 发布负责人 | 登记同一最终 SHA 的成功 run URL，包含合成 backup/restore |
| Production Deploy | 未完成 | 平台运维 | 登记成功 run URL、发布时间和 operator |
| Production Health Check | 未完成 | 平台运维 | 登记 `/healthz=200`、`/readyz=200` 的 run URL 与时间 |
| AuthZ gRPC 安全消息 | 未完成 | 应用运维 | 登记内部错误只返回 `Internal / internal server error` 的验证结果，不登记底层错误文本 |

## 5. 数据库备份证据

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| 宿主机 MySQL 客户端 | 未完成 | 平台运维 | `mysql`、`mysqldump` 均存在且为 8.x；只登记版本和检查时间 |
| 5.4.1-B 前手动备份 | 未完成 | 平台运维 | 登记 run ID、文件数量、总字节、目录 `0700`、文件 `0600`、`gzip -t=pass` |
| 5.4.1-B 后手动 `status` / `backup` | 未完成 | 平台运维 | 登记两个 run ID 与成功结论 |
| 首次后续定时备份 | 未完成 | 平台运维 | 登记 run ID、文件数量、总字节和 `gzip -t=pass` |

## 6. 5.4 安全处置历史证据

| 项目 | 当前状态 | 责任方 | 完成条件与安全元数据 |
| --- | --- | --- | --- |
| Refresh Token purge | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、dry-run matched 数量和结果；不得再次执行 purge 来制造历史证据 |
| 修复前 IAM 日志处置 | 未完成：历史证据尚未在仓库中登记 | 安全负责人 | 仅登记时间、operator、ticket/run ID、文件数量、总字节和结果；不得读取或复制正文 |
| 一个 Access Token TTL 观察 | 未完成：历史证据尚未在仓库中登记 | 应用运维 | 登记观察起止时间和“无新增 Token/SQL/raw gRPC error 泄露”的结论 |

历史证据确实无法取得时，本记录保持未完成，并由安全负责人决定是否另开维护窗口；本轮不默认授权任何破坏性操作。

## 7. 关闭条件

只有第 4、5、6 节所有必需项都有真实、安全的元数据证据，且同一最终 SHA 的 CI、CodeQL、MySQL 8、生产部署与健康检查均成功，才可把本记录的“当前结论”改为“最终验收完成”。
