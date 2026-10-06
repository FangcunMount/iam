# IAM Go SDK v5 迁移说明

本次升级要求 IAM 服务端、SDK、QS 和管理前端配套切换，所有用户重新登录。旧版本迁移记录保存在仓库的历史文档中。

## 接口版本

| 接口 | 新版本 |
| --- | --- |
| Go module | `github.com/FangcunMount/iam/v5`，`v5.0.0` |
| AuthN REST | `/api/v3/authn` |
| AuthN gRPC | `iam.authn.v3` |
| AuthZ REST | `/api/v4/authz` |
| AuthZ gRPC | `iam.authz.v4` |
| Identity、IDP | 保持 v2 |
| JWKS | 保持 `/api/v2/.well-known/jwks.json` |

AuthN REST 登录客户端位于 `pkg/sdk/auth/loginv3`。其他公开入口包括 `pkg/sdk`、`config`、`auth/client`、`auth/jwks`、`auth/verifier`、`authz`、`identity`、`idp` 和 `errors`。旧主版本的入口不提供别名。

## 登录、声明与验证

身份核验、登录准入、创建会话和颁发令牌是四个独立环节。登录请求和令牌声明不再包含授权分区。用户名使用 default Realm；外部身份提供方 Realm 和业务 OrgID 保持原义。

本地和远程验证必须配置非空 ExpectedAudience。IAM 使用 iam-api，QS 和 collection 分别使用 qs-api、collection-api。多个期望受众保持任意一个匹配语义；空列表、空元素均为参数错误。不从令牌内容推导期望受众。

业务组织读取 `TokenClaims.OrgID` 或 `BusinessOrgID()`，这是签名声明，不证明当前QS组织成员资格；在线Verify不从Session覆盖它，Refresh投影是另一条链。未知扩展字段不会被解释为组织或授权事实。角色和权限不放入JWT；请使用当前AuthZ策略检查，并另执行业务身份与对象准入。

切换时旧签名密钥退役，旧 Session 与 RefreshToken 清理，因此本地验签、远程验证和刷新都不能继续使用旧登录态。

## 授权与角色分配

Check、Snapshot及Assignment写方法没有domain参数。Subject/Resource/Action用于动作Check，AppName用于Snapshot投影；对象条件已退役，非空legacy ObjectCtx输入被拒绝。资源标识中的业务模块段不是授权分区，不应删除。当前gRPC Scope与Scoped Replace仍使用org_id字段，其业务含义是公司范围，未改成company_id。

角色名称全局唯一，公开查询使用名称或 ID，内部关系使用 RoleID。原 platform/super_admin 改为 platform_admin；原普通 super_admin 保留名称；tenant_admin 改为 iam_admin。不要使用这些名称绕过权限校验。

Role DTO返回management_protection：standard或protected。REST受保护角色及其关联事实的管理需要原始操作权限与roles/manage_protected；角色继承已退役。普通Role不能获得覆盖固定敏感集合的Grant，具体保护类别与集合以当前规则为准，不能推导全部管理能力已有限委派。

QS批量替换只修改部署配置中的受管M，集合外分配保持不变；它不是按创建者隔离。Scoped Replace另按公司处理并要求期望版本，增量按Subject+Role撤销则覆盖全部公司。服务身份来自mTLS，ChangedBy只是委派/审计声明，不能认证为REST用户或声明admin。

User停用触发Session撤销，不自动清空Assignment；受信服务Check仍可能读取其保留能力。服务需要遵守在线用户准入与当前业务身份，不能用授权快照fresh代替。完整时序见[AuthZ模块协作](../../../docs/02-业务模块/03-AuthZ/07-模块边界-AuthZ与AuthN-Identity-Suggest.md)。

## 全量档案与缓存

全量列表能力为profiles/list_all；全量分支只检查search_by_mobile_all，不回退普通search_by_mobile。非全量分支才用search_by_mobile，范围由显式ProfileID、组织、owner按OR组合；默认ProfileID来自created_by查询，ProfileLink不自动授予搜索可见性，URL中的ProfileID也不提权。Suggest的范围没有消费Assignment门店Scope。

当前SDK没有内建AuthZ结果缓存。策略事件类型iam.authz.version_changed.v2、主题iam.authz.version.v2，载荷仅含全局版本，用于推动IAM Runtime重载；不保证全部实例或业务缓存已失效。宿主若自管缓存，需设计主体/应用/动作范围隔离、版本水位、迟到结果处理及陈旧预算，不能把user+app键或事件接收当成完整撤权协议。Suggest的visibility缓存与索引还有独立更新窗口，详见[Scope消费及版本边界](06-authz.md)。

## 切换与回滚

先备份、预检、迁移并核对权限差异，再启用新密钥、退役旧密钥、清理登录状态和全部相关缓存，最后更新全部实例及调用方。具体操作见仓库 AuthZ 的安全加固与发布验收文档。

不提供恢复默认授权分区或关闭受众校验的兼容开关。回滚需配套恢复版本、配置和授权数据库备份，保留已退役密钥状态，并继续要求重新登录。
