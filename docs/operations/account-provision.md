# 维护登录身份准备

> 状态：已实现 · 2026-10-06 对照 account-provision 和 Signup 事务修订。

## 维护结论

`account-provision` 复用正常 Signup 应用事务，只创建 User、用户名 LoginIdentity 和密码 Credential。账号存在不代表具备 Operator、岗位动作或公司 Scope；这些事实按已确认映射分别配置，并在真实业务入口验收。固定 10002 的跨系统流程由 [专用手册](reviewer-10002-provisioning.md) 拥有。

## 私有输入与执行

数据库环境为 `IAM_APISERVER_MYSQL_HOST`、`IAM_APISERVER_MYSQL_PORT`、`IAM_APISERVER_MYSQL_USERNAME`（或 USER）、`IAM_APISERVER_MYSQL_PASSWORD`、`IAM_APISERVER_MYSQL_DATABASE`（或 DBNAME），不是 QS 数据库。端口默认 3306，主机可包含端口。凭据只从受限环境注入。

输入包含 request_id、actor_id、username、name、reason、password；密码由执行方随机生成并受限交付。输入为私有普通文件，报告目录 0700，报告 0600 新建且不能覆盖。日志/报告不包含密码及密码摘要，不能把凭据放参数。Apply 的执行人必须是当前有效的平台根管理员，事务锁定策略版本保持授权检查稳定。

```sh
iam-maintenance account-provision preflight --input /private/account/input.json --report /private/account/preflight-new.json
iam-maintenance account-provision apply --input /private/account/input.json --fingerprint REVIEWED_FINGERPRINT --report /private/account/apply-new.json
```

先核对预演、输入和操作人，使用最新真实 fingerprint。该命令只写身份，不写授权 Outbox，不能套用 Scope 写入的 standard 模式参数。

## 保留 ID、重复与恢复

维护入口支持可选 user_id，省略则系统生成；公开注册不接受指定 ID。已有 User（含已删除记录）、登录身份或直接赋权引用占用该 ID 时拒绝，不接管、不改号。

同名身份只有本次执行标识、非秘密输入指纹、用户状态及保留密码均匹配，才报告 historical_completed。其他用户名冲突、后续改密或状态变化停止，不自动重置密码。原操作人与 request_id 保留审计，该状态仅证明身份开通历史。

输出/网络结果不明时先核对私有报告与实际身份，保留同一输入及执行标识；不更换请求 ID 绕过占用。范围未配置前保持无后台数据权限。QS 旧 OperationAccountService 适配器停用，不恢复删除的 AccountOnboarding RPC；用正式 Operator/授权入口继续后续配置。

## 验证和来源

`cmd/iam-maintenance/account_provision.go`、`maintenance/accountprovision` 定义参数、私有文件、指纹和保留 ID；真实 MySQL 专项覆盖固定 ID、占用拒绝、事务回滚、重复不改密及操作权限撤销。测试与数据库插入均不能替代账号登录、范围准入或人工审核。
