# 数据库迁移文档

## 概述

本项目使用 [golang-migrate](https://github.com/golang-migrate/migrate) 进行数据库版本管理和迁移。

## 迁移文件列表

### 000001_init_schema

**版本**: 1  
**日期**: 2025-10-31  
**描述**: 初始化数据库 Schema，包含所有模块的表结构

**包含的模块**:

- Identity - 身份域
- Authentication (Authn) - 认证模块
- Authorization (Authz) - 授权模块
- Identity Provider (IDP) - 身份提供商
- Platform / System - 平台系统表

**主要表**:

- `iam_users` - 用户表
- `iam_profiles` - 儿童档案表
- `iam_refs` - 监护关系表
- `iam_auth_accounts` - 认证账号表（旧版）
- `iam_auth_wechat_accounts` - 微信账号扩展表（旧版）
- `iam_auth_operation_accounts` - 运营账号凭证表（旧版）
- `iam_jwks_keys` - JWKS 密钥表
- `iam_authz_*` - 授权相关表
- 其他系统表

### 000002_refactor_authn_schema

**版本**: 2.1  
**日期**: 2025-11-05  
**描述**: 重构认证模块表结构，使其符合领域驱动设计（DDD）

**主要变更**:

1. **统一账户表** (`iam_authn_accounts`)
   - 替代旧的 `iam_auth_accounts` + `iam_auth_wechat_accounts`
   - 统一管理所有类型的第三方登录账户
   - 账户类型: `wc-minip`（微信小程序）, `wc-offi`（微信公众号）, `wc-com`（企业微信）, `opera`（运营后台）
   - 字段说明:
     - `type`: 账户类型
     - `app_id`: 应用ID（微信 appid、企业微信 corpid、运营后台为空）
     - `external_id`: 外部平台用户标识（openid、userid、username）
     - `unique_id`: 全局唯一标识（unionid、运营后台为空）
     - `profile`: 用户资料（JSON格式：昵称、头像等）
     - `meta`: 额外元数据（JSON格式）
     - `status`: 账户状态（0-禁用, 1-激活, 2-归档, 3-删除）

2. **统一凭据表** (`iam_authn_credentials`)
   - 替代旧的 `iam_auth_operation_accounts`
   - 统一管理所有类型的认证凭据
   - 凭据类型: `password`, `phone_otp`, `oauth_wx_minip`, `oauth_wecom`
   - 字段说明:
     - `account_id`: 关联账户ID
     - `type`: 凭据类型
     - `idp`: IDP类型（wechat、wecom、phone、NULL）
     - `idp_identifier`: IDP标识符（unionid、openid@appid、userid、+E164、空）
     - `app_id`: 应用ID（wechat=appid、wecom=corpid、NULL）
     - `material`: 凭据材料（PHC哈希格式的密码、其他类型为NULL）
     - `algo`: 算法（argon2id、bcrypt、NULL）
     - `params_json`: 参数JSON（微信 profile、企业微信 agentid 等）
     - `status`: 凭据状态（0-禁用, 1-启用）
     - `failed_attempts`: 失败尝试次数（仅 password）
     - `locked_until`: 锁定截止时间（仅 password）

3. **Token 审计表** (`iam_authn_token_audit`)
   - 替代旧的 `iam_auth_sessions` + `iam_auth_revoked_access_token`
   - Token 主存储迁移到 Redis，表仅用于审计和长期追踪
   - 记录 Token 签发和撤销历史

**删除的表**:

- `iam_auth_accounts` (已合并到 `iam_authn_accounts`)
- `iam_auth_wechat_accounts` (已合并到 `iam_authn_accounts`)
- `iam_auth_operation_accounts` (已合并到 `iam_authn_credentials`)
- `iam_auth_sessions` (Token 迁移到 Redis)
- `iam_auth_revoked_access_token` (合并到 `iam_authn_token_audit`)

**设计优势**:

- ✅ 符合领域驱动设计原则
- ✅ 统一的账户和凭据管理
- ✅ 支持多种认证方式（密码、OTP、OAuth）
- ✅ 灵活的扩展性（通过 JSON 字段）
- ✅ 性能优化（Redis 存储 Token）
- ✅ 完整的审计追踪

## 使用方法

### 查看当前版本

```bash
make db-version
```

### 执行迁移（升级到最新版本）

```bash
make db-migrate
```

### 回滚一个版本

```bash
make db-rollback
```

### 回滚到指定版本

```bash
migrate -path internal/pkg/migration/migrations \
        -database "mysql://user:pass@tcp(host:port)/dbname" \
        goto 1
```

## 数据迁移注意事项

### 从 v1 升级到 v2

⚠️ **重要**: 本迁移会删除旧表中的所有数据！

**生产环境迁移步骤**:

1. **备份数据库**

   ```bash
   mysqldump -u root -p iam > backup_before_v2.sql
   ```

2. **准备数据迁移脚本**（如果需要保留数据）

   ```sql
   -- 迁移账户数据示例
   INSERT INTO iam_authn_accounts (id, user_id, type, app_id, external_id, unique_id, status, created_at, updated_at)
   SELECT 
       a.id,
       a.user_id,
       CASE a.provider
           WHEN 'wechat' THEN 'wc-minip'
           WHEN 'operation' THEN 'opera'
           ELSE a.provider
       END as type,
       COALESCE(a.app_id, '') as app_id,
       a.external_id,
       COALESCE(w.union_id, '') as unique_id,
       a.status,
       a.created_at,
       a.updated_at
   FROM iam_auth_accounts a
   LEFT JOIN iam_auth_wechat_accounts w ON a.id = w.account_id;
   
   -- 迁移凭据数据示例
   INSERT INTO iam_authn_credentials (account_id, type, idp, idp_identifier, material, algo, status, created_at, updated_at)
   SELECT 
       o.account_id,
       'password',
       NULL,
       '',
       o.password_hash,
       o.algo,
       1,
       o.created_at,
       o.updated_at
   FROM iam_auth_operation_accounts o;
   ```

3. **在测试环境验证迁移**

   ```bash
   # 测试环境执行迁移
   make db-migrate
   
   # 验证数据完整性
   # 验证应用功能正常
   ```

4. **生产环境执行**
   - 选择低峰时段
   - 准备回滚方案
   - 监控迁移过程
   - 验证数据和功能

5. **如需回滚**

   ```bash
   make db-rollback
   # 或恢复备份
   mysql -u root -p iam < backup_before_v2.sql
   ```

## 迁移文件命名规范

```text
{version}_{description}.{up|down}.sql
```

- `version`: 迁移版本号（6位数字，如 000001, 000002）
- `description`: 迁移描述（小写字母+下划线）
- `up`: 升级脚本
- `down`: 降级/回滚脚本

## 最佳实践

1. **每次迁移前备份数据库**
2. **先在测试环境验证**
3. **编写详细的迁移说明**
4. **确保 up 和 down 脚本成对存在**
5. **避免在迁移中修改数据**（如需修改，单独编写数据迁移脚本）
6. **使用事务（如果数据库支持）**
7. **迁移后验证数据完整性**

## 故障排查

### 迁移失败（dirty state）

```bash
# 查看当前版本和状态
make db-version

# 手动修复（假设失败在版本 2）
migrate -path internal/pkg/migration/migrations \
        -database "mysql://user:pass@tcp(host:port)/dbname" \
        force 2

# 重新执行迁移
make db-migrate
```

### 回滚后恢复数据

如果回滚后需要恢复数据，使用之前的备份：

```bash
mysql -u root -p iam < backup_before_v2.sql
```

## 相关文档

- [golang-migrate 官方文档](https://github.com/golang-migrate/migrate)
- [数据库设计文档](../../docs/authn/)
- [领域模型文档](../../internal/apiserver/domain/authn/)
