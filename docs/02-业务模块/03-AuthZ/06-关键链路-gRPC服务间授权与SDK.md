# 关键链路：gRPC 服务间授权与 SDK

> 状态：已实现 · 本轮代码已实现，生产数据切换与业务验收另行记录。

服务身份、mTLS 和方法 ACL 仍是 RPC 的前置边界。caller service 与被判定 Subject 不同；服务调用资格不能替代用户动作权限。

`Check` 接收主体、资源与具体动作，在当前运行时快照上判定。旧 `object_context` 字段编号保留并弃用；非空内容返回 `InvalidArgument`。快照权限统一输出 `UNCONDITIONAL`，`missing_attribute_keys` 始终为空。旧枚举和消息保留编号，不复用。

SDK 普通 `Check` 与 `Allow` 继续使用。`CheckObject` 保留公共符号并标记弃用，调用即返回条件授权已退役错误，不发起远程调用，也不降级为普通 Check。

`GetAuthorizationSnapshot` 是动作能力投影，不能替代业务范围检查。旧条件模式不能被消费者当作动作已授权；运营端显示权限数据待刷新并禁用对应操作。缓存仍需遵守版本、新鲜度和撤权边界。

Assignment RPC 保留方法 ACL 与受管角色白名单两层限制。调用服务只能管理其获准角色集合，不能凭管理员用户或通配动作授权绕过服务白名单。

错误分别处理：输入合同非法返回 InvalidArgument；调用服务无访问资格返回权限错误；合法请求无 Grant 匹配得到拒绝决策；运行时不可用时不得默认放行。

参阅 [SDK AuthZ 文档](../../../pkg/sdk/docs/06-authz.md)、[写入链路](03-关键链路-授权写入与受管Assignment.md)、[维护手册](08-条件授权退役维护手册.md)。

## 受管人员退出的完整角色事实

`GetAuthorizationSnapshot` 默认保持应用过滤后的 `roles`、`direct_roles` 和权限语义。维护调用方可显式提交 `include_assignment_facts=true`，读取该主体全部直接角色的 ID、名称与管理保护类别，包括其他应用及全局角色。此能力复用受管 Assignment 调用方准入，仅向具备对应服务身份的调用方开放；它不授予角色撤销能力，也不改变授权结果。

调用方必须检查 `assignment_facts_complete=true`，缺失时停止退出，不能把旧服务返回的空字段解释为没有角色。事实来自当前已发布的不可变策略快照，须结合策略版本与暂停授权写入完成切换核对。QS 用它识别受保护角色及白名单之外的授权；普通业务快照无需请求此字段。
