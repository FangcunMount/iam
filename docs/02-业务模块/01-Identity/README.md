# Identity：身份事实与关系生命周期

> 状态：已实现 · 本目录维护当前身份模型、用例与跨模块边界；命令清单是验证入口，不代表已取得运行或业务验收证据。

## 30 秒结论

Identity 拥有稳定主体 User、业务服务对象 Profile，以及两者之间有独立生命周期的 ProfileLink。注册、登录与建档是不同用例；身份关系是业务事实，不能直接推出全部资源动作权限。

当前需要记住：User 可没有 self Profile；建档在一个 MySQL 事务中创建 Profile 与 Link；未软删除 User 的非空 Phone 由数据库唯一约束保护；
Block/Deactivate 同事务保存状态与 Session 撤销意图的写入/去重结果，后台 Worker 才调用 AuthN；Suggest 以 SQL 定时派生有效关系投影。

## 阅读入口与事实所有者

| 要回答的问题 | Canonical 文档 |
| --- | --- |
| 模块负责什么、协议开放哪些能力 | [模块总览](00-模块总览.md) |
| 为什么拆分模型、字段与不变量由谁保护、哪些业务决策待确认 | [User / Profile / ProfileLink](01-领域模型-User-Profile-ProfileLink.md) |
| 创建输入、组合建档、事务归属、实际错误、操作者审计与重试结果 | [创建 User 与 Profile](02-关键链路-创建User与Profile.md) |
| 新建/恢复/撤销周期、REST与服务查询、分页、部分成功及旧请求重试 | [ProfileLink 链路](03-关键链路-建立与撤销ProfileLink.md) |
| signup 共享/借用事务、状态代次与撤销领取、/me写后失败、投影消费与候选合同 | [跨模块边界](04-模块边界-Identity与AuthN-AuthZ-Suggest.md) |
| 修改入口、实际装配/接口、历史迁移起点与检查覆盖 | [分层架构与代码索引](05-分层架构与代码索引.md) |

总览给入口，不重复完整设计论证；模型维护实体语义；链路维护步骤与失败；边界维护跨模块依赖；索引只负责定位。修订同一规则时优先更新对应正文，再核对摘要与图。

## 对外能力和安全边界

REST 提供当前 User 的查询/更新；档案查改要求当前 active ProfileLink，自身关系列表可包含撤销记录，指定 Profile 的 pair 查询仍先检查 active。REST 不提供档案全部关联人的列表。创建、状态命令、建立/撤销和批处理由 gRPC 提供，不能从 application 类型推导 REST 也开放同名命令。

- `ProfileLink.Revoke` 在同一实体实例上幂等；公开撤销要求 active link，重复请求会报错。
- 同 pair/type 撤销后可由 Establish 复用原 ID 恢复；恢复覆盖旧建立/撤销状态，including-revoked 查询不提供完整周期历史。
- User 状态与 Stage 写入/去重原子提交，不保证新增pending或Redis Session已撤销；User版本推进、旧任务、Worker运行与在途登录边界见[模块边界](04-模块边界-Identity与AuthN-AuthZ-Suggest.md)。
- `/me` roles 失败可降级；当前 runtime 的 permissions 失败会使 GET/PATCH 响应失败，PATCH 此时可能已保存资料。
- ProfileLink 局部业务检查与 AuthZ Resource/Action 能力要显式组合；User 联系资料不自动绑定登录入口，self 也没有 Profile 方向的唯一认领约束，模型中的声明不等于实名/关系证明。旧 object_context 不再是 IAM 授权输入。

这些边界的完整理由和失败矩阵由上表指定正文维护。

## 事实源与验证

当前代码和运行行为优先于迁移/契约，随后是测试、现行文档、历史材料。关键事实源为 `internal/apiserver/domain/identity`、`application/identity`、
`infra/mysql/{user,profile,profilelink,uow/identity,sessionrevocation}`，以及 Identity REST/proto、transport 与 container。

```bash
go test ./internal/apiserver/domain/identity/... ./internal/apiserver/application/identity/...
go test ./internal/apiserver/transport/rest/identity/... ./internal/apiserver/transport/grpc/service/identity
go test ./internal/apiserver/container/identity ./internal/apiserver/infra/mysql/sessionrevocation
make docs-hygiene
make docs-facts
```

MySQL 8 迁移约束、运行中的 Worker、真实调用方权限检查和业务验收应分别记录证据。全局维护规则见 [CONTRIBUTING-DOCS.md](../../CONTRIBUTING-DOCS.md)。
