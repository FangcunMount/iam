# IAM 项目深入代码库探索报告

**项目**: IAM - 企业级身份与访问管理平台  
**架构**: 六边形架构 + DDD (领域驱动设计) + CQRS (命令查询责任分离)  
**时间**: 2026年4月21日  
**探索广度**: Thorough (全面深入)

---

## 📋 目录

1. [项目整体架构](#1-项目整体架构)
2. [核心模块架构](#2-核心模块架构)
3. [关键业务域详解](#3-关键业务域详解)
4. [技术架构与基础设施](#4-技术架构与基础设施)
5. [设计模式与实践](#5-设计模式与实践)
6. [主要代码文件结构](#6-主要代码文件结构)
7. [核心实现代码片段](#7-核心实现代码片段)
8. [集成与边界](#8-集成与边界)
9. [当前关键特性](#9-当前关键特性)

---

## 1. 项目整体架构

### 1.1 运行时总体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                       外部客户端                                  │
│  Web / 小程序 / 管理后台 / 内部服务（QS/其他）                  │
└────────────┬──────────────────────────────────────┬──────────────┘
             │                                      │
             ▼                                      ▼
    ┌──────────────────┐              ┌──────────────────┐
    │   REST API       │              │   gRPC API       │
    │ /api/v1/authn/*  │              │  Service.proto   │
    │ /api/v1/authz/*  │              │                  │
    │ /api/v1/identity │              │  mTLS 保护       │
    └──────────┬───────┘              └────────┬─────────┘
               │                               │
               └───────────────┬───────────────┘
                               │
        ┌──────────────────────┴──────────────────────┐
        │        iam-apiserver (单一运行单元)          │
        │                                             │
        ├─────────────────────────────────────────────┤
        │  Interface Layer (REST/gRPC Adapters)       │
        │  ├─ authn/restful/handler/*                 │
        │  ├─ authn/grpc/service.go                   │
        │  ├─ authz/restful/handler/*                 │
        │  ├─ uc/restful/handler/*                    │
        │  └─ idp/restful/handler/*                   │
        ├─────────────────────────────────────────────┤
        │  Application Layer (业务编排 + CQRS)        │
        │  ├─ authn/(login,token,session,jwks)        │
        │  ├─ authz/(role,resource,policy,assign)     │
        │  ├─ uc/(user,child,guardianship)            │
        │  ├─ idp/(wechat 配置)                       │
        │  └─ suggest/(搜索)                          │
        ├─────────────────────────────────────────────┤
        │  Domain Layer (DDD 领域模型)                │
        │  ├─ Aggregates: User, Role, Policy, etc     │
        │  ├─ Entities: Account, Credential, etc      │
        │  ├─ Value Objects: Token, Principal, etc    │
        │  ├─ Domain Services: Authenticater, etc     │
        │  └─ Driven Ports: Repository interfaces     │
        ├─────────────────────────────────────────────┤
        │  Infrastructure Layer (技术实现)             │
        │  ├─ mysql/: 数据库仓储实现                  │
        │  ├─ redis/: 缓存实现                        │
        │  ├─ casbin/: 授权引擎                       │
        │  ├─ jwt/: JWT 签发与验证                    │
        │  ├─ crypto/: 加密算法                       │
        │  ├─ wechat/: 微信集成                       │
        │  └─ authentication/: 认证策略实现            │
        ├─────────────────────────────────────────────┤
        │  Container & Assembler (模块装配)           │
        │  ├─ assembler/authn.go                      │
        │  ├─ assembler/authz.go                      │
        │  ├─ assembler/uc.go                         │
        │  └─ 依赖注入与模块初始化                    │
        └─────────────────────────────────────────────┘
               │                │                │
               ▼                ▼                ▼
        ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
        │   MySQL      │ │    Redis     │ │   其他外部   │
        │   数据库     │ │    缓存      │ │   服务       │
        │              │ │              │ │ (Casbin等)  │
        └──────────────┘ └──────────────┘ └──────────────┘
```

### 1.2 主运行单元

**`iam-apiserver` 是唯一的中央进程**，不是多服务分散架构，承载了：

| 能力 | 实现方式 | 代码位置 |
|------|--------|---------|
| REST 暴露 | Gin Router 注册 | [internal/apiserver/routers.go](internal/apiserver/routers.go) |
| gRPC 暴露 | gRPC Server 注册 | [internal/apiserver/server.go](internal/apiserver/server.go) |
| Swagger | 代码注释生成 | [internal/apiserver/docs/swagger.yaml](internal/apiserver/docs/swagger.yaml) |
| 模块装配 | Container 集中初始化 | [internal/apiserver/container/](internal/apiserver/container/) |

### 1.3 对外暴露面全景

| 暴露方式 | 能力 | 对应模块 |
|---------|-----|--------|
| **REST** | 认证 (authn)、授权 (authz)、身份 (identity)、IDP、联想搜索 (suggest) | `/api/v1/authn/*`, `/api/v1/authz/*`, `/api/v1/identity/*` |
| **gRPC** | 认证、JWKS、授权、身份、IDP | [api/grpc/iam](api/grpc/iam) |
| **JWKS** | 公钥发布 | `/.well-known/jwks.json` |
| **SDK** | 统一接入层 | [pkg/sdk/](pkg/sdk/) |

---

## 2. 核心模块架构

### 2.1 四层分层设计（六边形架构）

```
┌────────────────────────────────────────────┐
│        Interface Layer (驱动适配器)         │
│  REST Handlers / gRPC Services / Middleware │
│  职责: 协议转换、参数绑定、响应转换       │
└──────────┬─────────────────────────────────┘
           │
           ▼
┌────────────────────────────────────────────┐
│      Application Layer (应用服务)          │
│  CommandService / QueryService / DTO       │
│  职责: 用例编排、事务边界、CQRS 分离      │
└──────────┬─────────────────────────────────┘
           │
           ▼
┌────────────────────────────────────────────┐
│       Domain Layer (领域模型)              │
│  Aggregates / Entities / Value Objects     │
│  Domain Services / Validators              │
│  Driven Ports (Repository Interfaces)      │
│  职责: 业务规则、领域逻辑、不变量维护     │
└──────────┬─────────────────────────────────┘
           │
           ▼
┌────────────────────────────────────────────┐
│    Infrastructure Layer (基础设施)        │
│  MySQL Repository / Redis Cache / Casbin   │
│  JWT Generator / Crypto / Third-party SDK │
│  职责: 技术细节、外部系统集成              │
└────────────────────────────────────────────┘
```

### 2.2 各层的真实代码位置

| 层级 | 目录 | 职责 | 代表文件 |
|-----|------|------|--------|
| Interface | `interface/*/restful/handler/` `interface/*/grpc/service.go` | 接协议、参数绑定、响应转换 | [interface/authn/restful/handler/auth.go](internal/apiserver/interface/authn/restful/handler/auth.go) |
| Application | `application/authn/login/` `application/authz/role/` `application/uc/user/` | 用例编排、CQRS 分离 | [application/authn/login/services.go](internal/apiserver/application/authn/login/services.go) |
| Domain | `domain/authn/authentication/` `domain/authz/role/` `domain/uc/user/` | 聚合、值对象、验证器、仓储接口 | [domain/authn/authentication/repository.go](internal/apiserver/domain/authn/authentication/repository.go) |
| Infrastructure | `infra/mysql/` `infra/redis/` `infra/casbin/` `infra/jwt/` | 仓储实现、缓存、策略引擎 | [infra/mysql/account/](internal/apiserver/infra/mysql/account/) |

---

## 3. 关键业务域详解

### 3.1 认证域 (authn)

#### 业务概述

负责证明"谁在什么租户下，以何种方式完成了认证"，颁发可消费的 Token。

#### 核心对象关系图

```
┌─────────────┐
│   Account   │  账户锚点 (可登录账户)
│ user_id(FK) │
│ provider    │
└────┬────────┘
     │ 关联多种凭据
     ▼
┌──────────────┐
│ Credential   │  凭据 (密码/OTP/OAuth)
│ account_id(FK)│
│ type         │
└──────────────┘
     │
     │ 认证判决
     ▼
┌──────────────┐
│ Principal    │  认证结果主体
│ user_id      │
│ account_id   │
└────┬─────────┘
     │ 创建会话
     ▼
┌──────────────┐
│  Session     │  会话 (运行时锚点)
│  sid         │
│  user_id(FK) │
└────┬─────────┘
     │ 颁发令牌
     ▼
┌──────────────────┐
│   TokenPair      │  令牌对
│ Access JWT       │
│ Refresh Token    │
│ (均带 sid)      │
└──────────────────┘
```

#### 关键应用服务

| 服务 | 位置 | 职责 |
|------|------|------|
| `LoginApplicationService` | [application/authn/login/services.go](internal/apiserver/application/authn/login/services.go) | 统一登录、多策略路由 |
| `TokenApplicationService` | [application/authn/token/services.go](internal/apiserver/application/authn/token/services.go) | Token 刷新、撤销、验证 |
| `SessionApplicationService` | [application/authn/session/services.go](internal/apiserver/application/authn/session/services.go) | Session 生命周期管理 |
| `KeyManagementAppService` | [application/authn/jwks/](internal/apiserver/application/authn/jwks/) | JWKS 密钥管理、轮换 |

#### 存储映射

```sql
-- 核心表
users (user_id, name, phone, email, ...)
auth_accounts (account_id, user_id, provider, external_id, status, ...)
auth_credentials (credential_id, account_id, type, material, ...)
jwks_keys (kid, status, public_key, private_key, ...)

-- Redis 存储
session:{sid} -> Principal JSON
refresh_token:{rtid} -> Token 元数据
revoked_access_token:{jti} -> 撤销记录
user_session_index:{user_id} -> [sid1, sid2, ...]
```

#### 认证流程示例（密码登录）

```go
// 1. 接收登录请求
req := LoginRequest{
    AuthType: AuthTypePassword,
    Username: "user@example.com",
    Password: "secret",
}

// 2. 应用服务路由
loginService.Login(ctx, req)

// 3. 认证判决
authenticater := NewAuthenticater(...)
decision := authenticater.AuthenticatePassword(
    ctx, username, password,
)

// 4. 创建 Principal
principal := NewPrincipal(decision.AccountID, decision.UserID, ...)

// 5. 创建 Session
sessionManager.CreateSession(ctx, principal)

// 6. 颁发令牌对
issuer := NewTokenIssuer(jwtGen, sessionStore, ...)
tokenPair := issuer.IssueTokenPair(ctx, session)

// 返回给客户端
return LoginResult{
    Principal: principal,
    TokenPair: tokenPair,
    UserID: decision.UserID,
}
```

#### 关键代码片段

**Domain: 认证仓储接口** ([domain/authn/authentication/repository.go](internal/apiserver/domain/authn/authentication/repository.go))
```go
type CredentialRepository interface {
    FindPasswordCredential(ctx context.Context, accountID meta.ID) (credentialID meta.ID, passwordHash string, err error)
    FindPhoneOTPCredential(ctx context.Context, phoneE164 string) (accountID, userID, credentialID meta.ID, err error)
    FindOAuthCredential(ctx context.Context, idpType, appID, idpIdentifier string) (accountID, userID, credentialID meta.ID, err error)
}

type AccountRepository interface {
    FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*UsernameLoginLookup, error)
    GetAccountStatus(ctx context.Context, accountID meta.ID) (enabled, locked bool, err error)
}
```

### 3.2 授权域 (authz)

#### 业务概述

负责回答"某主体在某租户下是否可以对某资源执行某动作"，管理角色、资源、策略和分配。

#### 核心对象模型

```
┌──────────────┐
│    Subject   │  主体 (user / group)
│ (from authn) │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│ Assignment   │  分配 (S2R 绑定)
│ sub_type     │
│ sub_id       │
│ role_id(FK)  │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│    Role      │  角色
│ tenant_id(FK)│
│ name         │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│ PolicyRule   │  策略规则
│ role_id(FK)  │
│ resource(FK) │
│ action       │
└────┬─────────┘
     │
     ▼
┌──────────────┐
│   Resource   │  资源
│ key (global) │
│ actions      │
└──────────────┘
     │
     └─→ Casbin(p/g rules)
```

#### 核心应用服务

| 服务 | 位置 | 职责 |
|------|------|------|
| `RoleCommandService` | [application/authz/role/command_service.go](internal/apiserver/application/authz/role/command_service.go) | 角色 CRUD |
| `RoleQueryService` | [application/authz/role/query_service.go](internal/apiserver/application/authz/role/query_service.go) | 角色查询 |
| `PolicyCommandService` | [application/authz/policy/command_service.go](internal/apiserver/application/authz/policy/command_service.go) | 策略修改 + Casbin 同步 |
| `AssignmentCommandService` | [application/authz/assignment/command_service.go](internal/apiserver/application/authz/assignment/command_service.go) | 分配管理 + Casbin 同步 |

#### CQRS 实践

**authz 是当前最清晰的 CQRS 落地**：

- 管理操作走 `*CommandService`（创建/修改/删除）
- 查询操作走 `*QueryService`（只读）
- Casbin 规则与业务表分别管理

```go
// Command 路径
commander.CreateRole(ctx, cmd)
commander.AddPolicy(ctx, rule)

// Query 路径
queryer.GetRole(ctx, roleID)
queryer.ListRoles(ctx, filter)

// Casbin 独立管理
casbinAdapter.AddPolicy(ctx, rule)
```

#### Casbin 模型配置

```ini
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

#### 权限判定流程

```
请求: (user:123, tenant:T1, resource:course, action:read)
    ▼
Casbin.Enforce(user:123, T1, course, read)
    ▼
查询用户分配: user:123 -> role:teacher
    ▼
查询角色策略: role:teacher, T1, course -> [read, write]
    ▼
匹配 action=read: ✓ 允许
```

### 3.3 用户域 (uc)

#### 业务概述

维护"人"和"儿童"这两类身份对象，以及它们之间的监护关系。

#### 核心对象模型

```
┌──────────────┐
│    User      │  用户 (成人、监护人)
│ user_id (PK) │
│ name         │
│ phone        │
│ status       │
└────┬─────────┘
     │ 关联监护关系
     │
     ▼
┌──────────────────┐
│ Guardianship     │  监护关系
│ user_id(FK)      │  (谁和哪个儿童是什么关系)
│ child_id(FK)     │
│ relation_type    │
│ established_at   │
│ revoked_at       │
└────┬─────────────┘
     │
     ▼
┌──────────────┐
│    Child     │  儿童 (被测评者)
│ child_id(PK) │
│ name         │
│ gender       │
│ birthday     │
│ id_card      │
└──────────────┘
```

#### 关键应用服务

| 服务 | 位置 | 职责 |
|------|------|------|
| `UserApplicationService` | [application/uc/user/services.go](internal/apiserver/application/uc/user/services.go) | 用户注册、资料更新 |
| `UserQueryApplicationService` | [application/uc/user/services.go](internal/apiserver/application/uc/user/services.go) | 用户查询 |
| `ChildApplicationService` | [application/uc/child/](internal/apiserver/application/uc/child/) | 儿童档案管理 |
| `GuardianshipApplicationService` | [application/uc/guardianship/](internal/apiserver/application/uc/guardianship/) | 监护关系管理 |

#### CQRS 在用户域的体现

```go
// 写侧应用服务
userApp.Register(ctx, dto)
userApp.Rename(ctx, userID, newName)
childApp.RegisterChild(ctx, dto)

// 读侧应用服务 (QueryApplicationService)
userApp.GetByID(ctx, userID)
userApp.GetByPhone(ctx, phone)

// 内部实现：所有读写都通过 UnitOfWork + Repository
// 不是独立读库，而是"接口分离，存储共用"
```

#### 操作限制

查询与访问控制大多从 `Guardianship` 关系出发：

```go
// 获取当前用户的所有儿童
guardians := guardianshipRepo.FindByGuardian(ctx, userID)
children := make([]*Child, len(guardians))
for i, g := range guardians {
    children[i], _ = childRepo.FindByID(ctx, g.ChildID)
}
```

#### 关键代码片段

**Domain: 用户聚合根** ([domain/uc/user/user.go](internal/apiserver/domain/uc/user/user.go))
```go
type User struct {
    ID       meta.ID
    Name     string
    Nickname string
    Phone    meta.Phone
    Email    meta.Email
    IDCard   meta.IDCard
    Status   UserStatus
}

func NewUser(name string, phone meta.Phone, opts ...UserOption) (*User, error) {
    if name == "" {
        return nil, errors.WithCode(code.ErrUserBasicInfoInvalid, "name cannot be empty")
    }
    user := &User{
        Name:   name,
        Phone:  phone,
        Status: UserActive,
    }
    return user, nil
}

// 状态管理方法
func (u *User) Activate()   { u.Status = UserActive }
func (u *User) Deactivate() { u.Status = UserInactive }
func (u *User) Block()      { u.Status = UserBlocked }
```

### 3.4 IDP 模块 (身份提供商)

#### 业务概述

处理第三方认证集成（微信小程序、企业微信等）。

#### 核心组件

| 组件 | 位置 | 职责 |
|------|------|------|
| `WechatAppRepository` | [domain/idp/wechatapp/repository.go](internal/apiserver/domain/idp/wechatapp/repository.go) | 应用配置存储 |
| `AccessTokenCache` | [infra/redis/accesstoken_cache.go](internal/apiserver/infra/redis/accesstoken_cache.go) | WeChat Access Token 缓存 |
| `IdentityProvider` | [domain/authn/authentication/external.go](internal/apiserver/domain/authn/authentication/external.go) | OAuth/OIDC 端口 |

#### WeChat 集成流程

```
用户授权微信登录
    ▼
POST /api/v1/authn/login
    ▼
验证 WeChat Code（通过 IDP）
    ▼
获取 OpenID / UnionID
    ▼
查询或创建 OAuth Credential
    ▼
完成认证流程
```

### 3.5 联想搜索模块 (suggest)

#### 业务概述

提供儿童名称/拼音/手机的快速联想搜索。

#### 架构

```
MySQL 数据源
    ▼
Loader (Raw SQL)
    ▼
搜索项行 (name|id|mobiles|...)
    ▼
Store (内存 Trie + Hash 索引)
    ▼
REST GET /api/v1/suggest/child?k=
```

#### 关键组件

| 组件 | 位置 | 职责 |
|------|------|------|
| `Loader` | [infra/mysql/suggest/loader.go](internal/apiserver/infra/mysql/suggest/loader.go) | 从 MySQL 拉取原始数据 |
| `Updater` | [application/suggest/](internal/apiserver/application/suggest/) | 构建内存索引 |
| `Store` | [infra/suggest/search/](internal/apiserver/infra/suggest/search/) | Trie + Hash 搜索 |
| `Cron Scheduler` | [infra/scheduler/](internal/apiserver/infra/scheduler/) | 定时刷新 |

---

## 4. 技术架构与基础设施

### 4.1 技术栈

| 技术 | 用途 | 位置 |
|------|------|------|
| **Gin** | HTTP Web 框架 | 路由注册 |
| **gRPC** | RPC 框架 + Protobuf | 服务间通信 |
| **GORM** | ORM 框架 | [infra/mysql/](internal/apiserver/infra/mysql/) |
| **Redis** | 缓存 + Session 存储 | [infra/redis/](internal/apiserver/infra/redis/) |
| **Casbin** | RBAC 授权引擎 | [infra/casbin/adapter.go](internal/apiserver/infra/casbin/adapter.go) |
| **JWT (lestrrat-go)** | Token 生成与验证 | [infra/jwt/](internal/apiserver/infra/jwt/) |
| **protobuf** | 数据序列化 | [api/grpc/iam/](api/grpc/iam/) |

### 4.2 数据持久化

#### MySQL 数据模型

**认证表**
```sql
users
├─ user_id (PK)
├─ name
├─ phone
├─ email
├─ id_card
└─ status

auth_accounts
├─ account_id (PK)
├─ user_id (FK)
├─ provider
├─ external_id
├─ status
└─ locked

auth_credentials
├─ credential_id (PK)
├─ account_id (FK)
├─ type (password/otp/oauth)
└─ material (encrypted)

jwks_keys
├─ kid (PK)
├─ status
├─ public_key
└─ private_key
```

**授权表**
```sql
authz_roles
├─ role_id (PK)
├─ tenant_id (FK)
├─ name
└─ display_name

authz_resources
├─ resource_id (PK)
├─ key (unique)
└─ actions (JSON)

authz_assignments
├─ assignment_id (PK)
├─ subject_type
├─ subject_id
├─ role_id (FK)
├─ tenant_id (FK)
└─ assigned_at

authz_policy_versions
├─ version_id (PK)
├─ tenant_id (FK)
├─ policy_version
└─ updated_at

casbin_rule
├─ id (PK)
├─ ptype (p/g)
├─ v0-v3 (Casbin 规则)
```

**用户域表**
```sql
children
├─ child_id (PK)
├─ name
├─ gender
├─ birthday
├─ id_card
└─ status

guardianships
├─ guardianship_id (PK)
├─ user_id (FK)
├─ child_id (FK)
├─ relation_type
├─ established_at
├─ revoked_at
└─ (user_id, child_id unique)
```

#### Redis 存储结构

```
# Session 存储
session:{sid} -> Principal JSON
user_session_index:{user_id} -> {sid1, sid2, ...}
account_session_index:{account_id} -> {sid1, sid2, ...}

# Token 存储
refresh_token:{rtid} -> Token 元数据
access_token_store:{jti} -> Token 信息

# 撤销记录
revoked_access_token:{jti} -> 1 (TTL = token 剩余时间)
revoked_refresh_token:{rtid} -> 1 (TTL = token 剩余时间)

# JWKS 相关
jwks_keys_v:{version} -> JWKS JSON
jwks_keys_kid_index:{kid} -> version

# IDP 缓存
wechat_access_token:{app_id} -> AppAccessToken JSON

# 缓存治理
cache_family:{family_id} -> FamilyStatus JSON
```

### 4.3 基础设施实现

#### MySQL 仓储实现示例

[infra/mysql/account/repository.go] 结构：
```
└─ account/
   ├─ po.go (Persistent Object 数据模型)
   ├─ repository.go (Repository 接口实现)
   └─ mapper.go (PO <-> Domain Model 映射)
```

#### Redis 缓存实现

[infra/redis/] 结构：
```
├─ accesstoken_cache.go (WeChat AccessToken 缓存)
├─ session_store.go (Session 存储)
├─ token_store.go (Token 存储)
├─ otp_store.go (OTP 验证码缓存)
├─ cache_inspector.go (缓存监控)
└─ utils.go (键管理、序列化)
```

#### Casbin 适配器实现

[infra/casbin/adapter.go]:
```go
type CasbinAdapter struct {
    enforcer *casbin.CachedEnforcer
    mu       sync.RWMutex
}

func (c *CasbinAdapter) AddPolicy(ctx context.Context, rules ...domain.PolicyRule) error
func (c *CasbinAdapter) RemovePolicy(ctx context.Context, rules ...domain.PolicyRule) error
func (c *CasbinAdapter) AddGroupingPolicy(ctx context.Context, rules ...domain.GroupingRule) error
func (c *CasbinAdapter) Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error)
```

### 4.4 缓存治理系统

**缓存族 (Cache Family)** 概念用于统一管理各个缓存分类：

| 缓存族 | ID | 主要用途 |
|-------|----|---------| 
| Session | `authn_session` | 用户会话存储 |
| Token | `authn_token` | 令牌验证缓存 |
| JWKS | `authn_jwks` | 公钥缓存 |
| AccessToken | `idp_wechat_access_token` | 微信 Token 缓存 |
| OTP | `authn_otp` | 验证码缓存 |

**监控和治理** ([application/cachegovernance/](internal/apiserver/application/cachegovernance/)):
```go
// 获取缓存总体视图
overview, err := cacheService.Overview(ctx)

// 查询特定缓存族
family, err := cacheService.Family(ctx, "authn_session")

// 监控缓存健康状态
status := family.Status
```

---

## 5. 设计模式与实践

### 5.1 六边形架构（港口与适配器）

**原理**：业务逻辑与外部系统隔离，通过端口与适配器通信。

**实现**：

```
┌─────────────────────────────────────┐
│       Domain Ports (接口)           │
│  ├─ Repository interfaces          │
│  ├─ ExternalService interfaces     │
│  └─ 由 infra 提供实现 (adapters)   │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│      Driving Ports (驱动端)          │
│  ├─ REST handlers                  │
│  ├─ gRPC services                  │
│  └─ 接收外部请求                    │
└─────────────────────────────────────┘
```

**代码位置**：
- Driven Ports: [domain/authn/authentication/repository.go](internal/apiserver/domain/authn/authentication/repository.go)
- Driving Adapters: [interface/authn/restful/handler/](internal/apiserver/interface/authn/restful/handler/)
- 模块装配: [container/assembler/](internal/apiserver/container/assembler/)

### 5.2 CQRS 模式（命令查询责任分离）

**原理**：将"修改状态"和"查询状态"分离为不同的接口与服务。

**清晰落地的模块**：authz

```go
// 命令侧（写）
type RoleCommandService struct {
    CreateRole(ctx context.Context, cmd) error
    UpdateRole(ctx context.Context, cmd) error
    DeleteRole(ctx context.Context, roleID) error
}

// 查询侧（读）
type RoleQueryService struct {
    GetRole(ctx context.Context, roleID) (*Role, error)
    ListRoles(ctx context.Context, filter) ([]*Role, error)
}
```

**部分落地的模块**：uc

```go
// 写操作分散在多个服务
type UserApplicationService interface {
    Register(ctx context.Context, dto RegisterUserDTO) (*UserResult, error)
}

type UserProfileApplicationService interface {
    Rename(ctx context.Context, userID string, newName string) error
}

// 读操作集中
type UserQueryApplicationService interface {
    GetByID(ctx context.Context, userID string) (*UserResult, error)
    GetByPhone(ctx context.Context, phone string) (*UserResult, error)
}

// 内部实现：都通过 UnitOfWork 访问同一套 Repository
```

**优势**：
- 读写接口清晰分离
- 允许不同的优化策略（缓存、索引等）
- 易于单元测试

### 5.3 DDD 领域驱动设计

**应用**：

| DDD 概念 | 代码位置 | 例子 |
|---------|---------|------|
| **Aggregate (聚合根)** | `domain/*/model.go` | User, Role, Account |
| **Entity (实体)** | `domain/*/model.go` | Child (带生命周期) |
| **Value Object (值对象)** | `domain/authn/token/token.go` | Token, Principal, TokenClaims |
| **Aggregate Repository** | `domain/*/repository.go` | UserRepository, RoleRepository |
| **Domain Service** | `domain/authn/authentication/authenticater.go` | Authenticater (认证决策) |
| **Domain Event** | N/A (当前未用) | - |
| **Bounded Context (限界上下文)** | `domain/{authn,authz,uc,idp,suggest}` | 各模块独立上下文 |

### 5.4 依赖注入与模块装配

**方式**：构造函数注入 + 集中式 Container

**实现**：[internal/apiserver/container/](internal/apiserver/container/)

```go
// assembler/authn.go
type AuthnModule struct {
    AccountService       accountApp.AccountApplicationService
    LoginService         login.LoginApplicationService
    TokenService         token.TokenApplicationService
    // ... 其他服务
}

// 装配函数
func assembleAuthnModule(cfg *config.Config, db *gorm.DB, redis *redis.Client) *AuthnModule {
    // 创建基础设施
    accountRepo := acctrepo.NewRepository(db)
    credentialRepo := credentialrepo.NewRepository(db)
    sessionStore := redis.NewSessionStore(redis)
    
    // 创建应用服务
    loginService := login.NewLoginApplicationService(
        accountRepo,
        credentialRepo,
        // ...
    )
    
    return &AuthnModule{
        LoginService: loginService,
        // ...
    }
}
```

### 5.5 单元测试与可测性

**设计**：仓储接口明确，易于 Mock

```go
// 接口定义
type AccountRepository interface {
    FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*UsernameLoginLookup, error)
}

// 测试 Mock
type MockAccountRepository struct {
    mock.Mock
}

func (m *MockAccountRepository) FindAccountByUsername(...) (...) {
    args := m.Called(...)
    return args.Get(0).(*UsernameLoginLookup), args.Error(1)
}

// 测试用例
func TestLogin(t *testing.T) {
    mockRepo := new(MockAccountRepository)
    mockRepo.On("FindAccountByUsername", ...).Return(lookup, nil)
    
    service := NewLoginApplicationService(mockRepo, ...)
    result, err := service.Login(ctx, req)
    // ...
}
```

---

## 6. 主要代码文件结构

### 6.1 目录树概览

```
iam/
├── api/
│   ├── rest/
│   │   ├── authn.v1.yaml        # REST 契约
│   │   ├── authz.v1.yaml
│   │   ├── identity.v1.yaml
│   │   ├── idp.v1.yaml
│   │   └── suggest.v1.yaml
│   └── grpc/
│       └── iam/
│           ├── authn/v1/        # gRPC 服务定义
│           ├── authz/v1/
│           ├── identity/v1/
│           └── idp/v1/
├── internal/apiserver/
│   ├── interface/               # 适配器层
│   │   ├── authn/
│   │   │   ├── restful/handler/ # HTTP 处理器
│   │   │   └── grpc/            # gRPC 服务
│   │   ├── authz/
│   │   ├── uc/
│   │   ├── idp/
│   │   └── suggest/
│   ├── application/             # 应用层
│   │   ├── authn/
│   │   │   ├── login/           # 登录用例
│   │   │   ├── token/           # 令牌用例
│   │   │   ├── session/         # 会话管理
│   │   │   └── jwks/            # JWKS 管理
│   │   ├── authz/
│   │   │   ├── role/            # CQRS 示例
│   │   │   ├── policy/
│   │   │   ├── resource/
│   │   │   └── assignment/
│   │   ├── uc/
│   │   │   ├── user/
│   │   │   ├── child/
│   │   │   ├── guardianship/
│   │   │   └── uow/             # Unit of Work
│   │   ├── idp/
│   │   ├── suggest/
│   │   └── cachegovernance/     # 缓存治理
│   ├── domain/                  # 领域层
│   │   ├── authn/
│   │   │   ├── account/         # 聚合根
│   │   │   ├── authentication/  # 认证逻辑
│   │   │   ├── credential/      # 凭据
│   │   │   ├── session/         # 会话
│   │   │   ├── token/           # 令牌
│   │   │   └── jwks/            # 密钥
│   │   ├── authz/
│   │   │   ├── role/            # 聚合根
│   │   │   ├── policy/
│   │   │   ├── resource/
│   │   │   └── assignment/
│   │   ├── uc/
│   │   │   ├── user/            # 聚合根
│   │   │   ├── child/           # 聚合根
│   │   │   └── guardianship/    # 聚合根
│   │   ├── idp/
│   │   │   └── wechatapp/
│   │   └── suggest/
│   ├── infra/                   # 基础设施层
│   │   ├── mysql/
│   │   │   ├── account/         # 仓储实现
│   │   │   ├── credential/
│   │   │   ├── role/
│   │   │   ├── user/
│   │   │   ├── child/
│   │   │   └── guardianship/
│   │   ├── redis/
│   │   │   ├── session_store.go
│   │   │   ├── token_store.go
│   │   │   ├── otp_store.go
│   │   │   └── accesstoken_cache.go
│   │   ├── casbin/
│   │   │   └── adapter.go       # Casbin 适配器
│   │   ├── jwt/
│   │   │   ├── generator.go
│   │   │   └── verifier.go
│   │   ├── crypto/
│   │   ├── wechat/              # WeChat 客户端
│   │   ├── wechatapi/
│   │   ├── authentication/      # 认证策略
│   │   ├── cache/               # 缓存治理支持
│   │   ├── messaging/           # 消息队列
│   │   ├── scheduler/           # 定时任务
│   │   └── sms/                 # SMS 服务
│   ├── container/               # 模块装配
│   │   └── assembler/
│   │       ├── authn.go
│   │       ├── authz.go
│   │       ├── uc.go
│   │       ├── idp.go
│   │       └── suggest.go
│   ├── app.go                   # 应用入口
│   ├── routers.go               # 路由注册
│   ├── server.go                # 服务器启动
│   ├── run.go                   # 运行逻辑
│   ├── database.go              # 数据库连接
│   └── options/                 # 配置选项
├── pkg/
│   ├── sdk/                     # SDK 包装
│   │   ├── sdk.go
│   │   ├── auth/
│   │   ├── identity/
│   │   ├── authz/
│   │   ├── config/
│   │   └── transport/
│   ├── app/                     # 应用框架
│   ├── core/                    # 核心工具
│   └── middleware/              # 中间件
├── cmd/
│   └── apiserver/
│       └── apiserver.go         # 进程入口
├── configs/
│   ├── apiserver.dev.yaml
│   ├── apiserver.prod.yaml
│   ├── casbin_model.conf        # Casbin 模型
│   ├── grpc_acl.yaml            # gRPC ACL
│   ├── mysql/
│   │   └── schema.sql           # 数据库 schema
│   └── keys/                    # SSL 证书
├── docs/                        # 文档
│   ├── 00-概览/
│   ├── 01-运行时/
│   ├── 02-业务域/
│   ├── 03-接口与集成/
│   ├── 04-基础设施与运维/
│   └── 05-专题分析/
└── Makefile                     # 构建脚本
```

### 6.2 关键代码文件

| 文件 | 行数 | 职责 |
|------|------|------|
| [cmd/apiserver/apiserver.go](cmd/apiserver/apiserver.go) | ~100 | 进程主入口 |
| [internal/apiserver/app.go](internal/apiserver/app.go) | ~50 | 应用初始化 |
| [internal/apiserver/routers.go](internal/apiserver/routers.go) | ~200 | HTTP/gRPC 路由 |
| [internal/apiserver/server.go](internal/apiserver/server.go) | ~300+ | 服务器启动 |
| [internal/apiserver/container/container.go](internal/apiserver/container/container.go) | ~100 | 模块容器 |
| [internal/apiserver/application/authn/login/services.go](internal/apiserver/application/authn/login/services.go) | ~80 | 登录用例接口 |
| [internal/apiserver/application/authn/login/services_impl.go](internal/apiserver/application/authn/login/services_impl.go) | ~150+ | 登录用例实现 |
| [internal/apiserver/domain/authn/authentication/repository.go](internal/apiserver/domain/authn/authentication/repository.go) | ~60 | 认证仓储接口 |
| [internal/apiserver/infra/mysql/account/repository.go](internal/apiserver/infra/mysql/account/repository.go) | ~100+ | 账户仓储实现 |
| [internal/apiserver/infra/casbin/adapter.go](internal/apiserver/infra/casbin/adapter.go) | ~80+ | Casbin 适配器 |
| [internal/apiserver/interface/authn/restful/handler/auth.go](internal/apiserver/interface/authn/restful/handler/auth.go) | ~150+ | 认证 HTTP 处理 |
| [internal/pkg/middleware/authn/jwt_middleware.go](internal/pkg/middleware/authn/jwt_middleware.go) | ~150+ | JWT 中间件 |

---

## 7. 核心实现代码片段

### 7.1 登录流程完整示例

**请求流程**：REST Handler → Application Service → Domain → Infrastructure

```go
// 1. REST 处理器 (interface layer)
func (h *AuthHandler) Login(c *gin.Context) {
    var reqBody req.LoginRequest
    c.BindJSON(&reqBody)
    
    // 根据方法路由
    switch reqBody.Method {
    case "password":
        h.handlePasswordLogin(c, reqBody)
    case "wechat":
        h.handleWeChatLogin(c, reqBody)
    }
}

// 2. 应用服务 (application layer)
func (s *loginService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
    // 根据认证类型选择认证策略
    var decision *authentication.AuthDecision
    
    switch req.AuthType {
    case AuthTypePassword:
        decision, err = s.authenticatePassword(ctx, req)
    case AuthTypeWechat:
        decision, err = s.authenticateWechat(ctx, req)
    }
    
    if err != nil {
        return nil, err
    }
    
    // 创建主体
    principal := authentication.NewPrincipal(
        decision.AccountID,
        decision.UserID,
        decision.TenantID,
        decision.Attributes,
    )
    
    // 创建会话
    session, err := s.sessionManager.CreateSession(ctx, principal)
    if err != nil {
        return nil, err
    }
    
    // 颁发令牌
    tokenPair, err := s.tokenIssuer.IssueTokenPair(ctx, session)
    if err != nil {
        return nil, err
    }
    
    return &LoginResult{
        Principal: principal,
        TokenPair: tokenPair,
        UserID: decision.UserID,
    }, nil
}

// 3. 密码认证 (domain layer)
func (s *loginService) authenticatePassword(ctx context.Context, req LoginRequest) (*AuthDecision, error) {
    // 查询账户
    lookup, err := s.accountRepo.FindAccountByUsername(ctx, req.TenantID, *req.Username)
    if err != nil {
        return nil, err
    }
    
    // 检查账户状态
    enabled, locked, err := s.accountRepo.GetAccountStatus(ctx, lookup.AccountID)
    if locked || !enabled {
        return nil, ErrAccountLocked
    }
    
    // 查询凭据
    credID, passwordHash, err := s.credentialRepo.FindPasswordCredential(ctx, lookup.AccountID)
    if err != nil {
        return nil, err
    }
    
    // 验证密码
    if !crypto.VerifyPassword(*req.Password, passwordHash) {
        return nil, ErrAuthenticationFailed
    }
    
    // 返回认证决策
    return &AuthDecision{
        AccountID: lookup.AccountID,
        UserID: lookup.UserID,
        TenantID: lookup.ScopedTenantID,
        Attributes: map[string]string{"amr": "pwd"},
    }, nil
}
```

### 7.2 权限检查流程

```go
// 1. Handler 接收请求
func (h *CheckHandler) Check(c *gin.Context) {
    var req CheckRequest
    c.BindJSON(&req)
    
    // 调用应用服务
    result, err := h.checkService.Check(c.Request.Context(), req)
    c.JSON(200, result)
}

// 2. 应用服务（可能涉及缓存）
func (s *CheckService) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
    // 尝试从缓存读
    cached, found := s.cache.Get(req.Subject, req.Domain, req.Object, req.Action)
    if found {
        return cached, nil
    }
    
    // 执行 Enforce
    allowed, err := s.casbinAdapter.Enforce(
        ctx,
        req.Subject,
        req.Domain,
        req.Object,
        req.Action,
    )
    if err != nil {
        return nil, err
    }
    
    // 缓存结果
    result := &CheckResult{Allowed: allowed}
    s.cache.Set(req.Subject, req.Domain, req.Object, req.Action, result, ttl)
    
    return result, nil
}

// 3. Casbin 适配器执行
func (c *CasbinAdapter) Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    return c.enforcer.Enforce(sub, dom, obj, act)
}

// Casbin 内部流程：
// 1. 查询用户分配: user:123 -> role:teacher (g 规则)
// 2. 查询角色策略: role:teacher, domain, object -> [read, write] (p 规则)
// 3. 匹配 action=read: ✓ 允许
```

### 7.3 Module 装配示例

```go
// container/assembler/authn.go
func AssembleAuthnModule(cfg *config.Config, db *gorm.DB, redis *redis.Client) *AuthnModule {
    // ======== Infrastructure 层 ========
    accountRepo := acctrepo.NewRepository(db)
    credentialRepo := credentialrepo.NewRepository(db)
    jwksRepo := jwksMysql.NewRepository(db)
    
    sessionStore := redisInfra.NewSessionStore(redis)
    tokenStore := redisInfra.NewTokenStore(redis)
    otpStore := redisInfra.NewOTPVerifierImpl(redis)
    
    jwtGen := jwtinfra.NewJWTGenerator(jwksRepo)
    crypto := crypto.NewPasswordHasher()
    
    // ======== Domain 层 ========
    authenticater := authentication.NewAuthenticater(
        credentialRepo,
        accountRepo,
        jwtGen,
        crypto,
        // ...
    )
    
    sessionManager := sessionDomain.NewSessionManager(sessionStore)
    tokenIssuer := tokenDomain.NewTokenIssuer(jwtGen, sessionManager, tokenStore)
    
    // ======== Application 层 ========
    loginService := login.NewLoginApplicationService(
        accountRepo,
        credentialRepo,
        authenticater,
        sessionManager,
        tokenIssuer,
    )
    
    tokenService := token.NewTokenApplicationService(
        tokenStore,
        sessionStore,
        jwtGen,
        tokenIssuer,
    )
    
    // ======== Interface 层 ========
    authHandler := authhandler.NewAuthHandler(loginService, tokenService, ...)
    
    // ======== 返回模块 ========
    return &AuthnModule{
        LoginService: loginService,
        TokenService: tokenService,
        AuthHandler: authHandler,
        // ...
    }
}
```

### 7.4 CQRS 实践 - 角色管理

```go
// ======== Command 路径 ========
type RoleCommandService struct {
    roleValidator roleDomain.Validator
    roleRepo      roleDomain.Repository
    casbinAdapter domain.CasbinAdapter
}

func (s *RoleCommandService) CreateRole(ctx context.Context, cmd roleDomain.CreateRoleCommand) (*roleDomain.Role, error) {
    // 1. 验证命令
    if err := s.roleValidator.ValidateCreateCommand(cmd); err != nil {
        return nil, err
    }
    
    // 2. 创建角色对象
    newRole := roleDomain.NewRole(
        cmd.Name,
        cmd.DisplayName,
        cmd.TenantID,
        roleDomain.WithDescription(cmd.Description),
    )
    
    // 3. 持久化
    if err := s.roleRepo.Create(ctx, &newRole); err != nil {
        return nil, err
    }
    
    return &newRole, nil
}

func (s *RoleCommandService) AssignPolicy(ctx context.Context, roleID string, policy roleDomain.PolicyRule) error {
    // 1. 获取角色
    role, err := s.roleRepo.FindByID(ctx, roleID)
    if err != nil {
        return err
    }
    
    // 2. 更新角色（业务逻辑）
    role.AddPolicy(policy)
    
    // 3. 持久化角色变更
    if err := s.roleRepo.Update(ctx, role); err != nil {
        return err
    }
    
    // 4. 同步 Casbin 规则
    casbinRule := domain.PolicyRule{
        Sub: role.Key(),
        Dom: policy.Domain,
        Obj: policy.Object,
        Act: policy.Action,
    }
    
    if err := s.casbinAdapter.AddPolicy(ctx, casbinRule); err != nil {
        return err
    }
    
    return nil
}

// ======== Query 路径 ========
type RoleQueryService struct {
    roleRepo roleDomain.Repository
    cache    cache.Cache
}

func (s *RoleQueryService) GetRole(ctx context.Context, roleID string) (*roleDomain.Role, error) {
    // 1. 尝试缓存
    if cached, found := s.cache.Get("role:" + roleID); found {
        return cached.(*roleDomain.Role), nil
    }
    
    // 2. 查询仓储
    role, err := s.roleRepo.FindByID(ctx, roleID)
    if err != nil {
        return nil, err
    }
    
    // 3. 缓存结果
    s.cache.Set("role:"+roleID, role, ttl)
    
    return role, nil
}

func (s *RoleQueryService) ListRoles(ctx context.Context, filter roleDomain.ListFilter) ([]*roleDomain.Role, error) {
    return s.roleRepo.FindByFilter(ctx, filter)
}

// ======== Handler 中的使用 ========
func (h *RoleHandler) CreateRole(c *gin.Context) {
    var cmd roleDomain.CreateRoleCommand
    c.BindJSON(&cmd)
    
    // 调用 command 服务
    role, err := h.commander.CreateRole(c.Request.Context(), cmd)
    c.JSON(201, role)
}

func (h *RoleHandler) GetRole(c *gin.Context) {
    roleID := c.Param("roleId")
    
    // 调用 query 服务
    role, err := h.queryer.GetRole(c.Request.Context(), roleID)
    c.JSON(200, role)
}
```

### 7.5 Unit of Work 模式 - 用户域

```go
// application/uc/uow/uow.go
type UnitOfWork struct {
    db          *gorm.DB
    repositories *TxRepositories
}

type TxRepositories struct {
    Users         user.Repository
    Children      child.Repository
    Guardianships guardianship.Repository
}

// 事务边界管理
func (uow *UnitOfWork) WithinTx(ctx context.Context, fn func(*TxRepositories) error) error {
    tx := uow.db.BeginTx(ctx, nil)
    if tx.Error != nil {
        return tx.Error
    }
    
    // 为事务内的操作创建仓储
    txRepos := &TxRepositories{
        Users:         user.NewRepository(tx),
        Children:      child.NewRepository(tx),
        Guardianships: guardianship.NewRepository(tx),
    }
    
    // 执行业务逻辑
    if err := fn(txRepos); err != nil {
        tx.Rollback()
        return err
    }
    
    // 提交事务
    return tx.Commit().Error
}

// 使用示例
func (s *GuardianshipService) RegisterChild(ctx context.Context, dto RegisterChildDTO) error {
    return s.uow.WithinTx(ctx, func(repos *TxRepositories) error {
        // 1. 创建儿童档案
        child := child.NewChild(dto.Name, dto.Gender, dto.Birthday)
        if err := repos.Children.Create(ctx, child); err != nil {
            return err
        }
        
        // 2. 创建监护关系
        guardianship := guardianship.NewGuardianship(
            dto.UserID,
            child.ID,
            guardianship.RelationTypeParent,
        )
        if err := repos.Guardianships.Create(ctx, guardianship); err != nil {
            return err
        }
        
        return nil
    })
}
```

---

## 8. 集成与边界

### 8.1 与 QS 系统的集成

#### 推荐接入路径

1. **JWT 验证**：优先本地 JWKS 验签
   ```go
   jwksManager, _ := authjwks.NewJWKSManager(&sdk.JWKSConfig{
       URL: "http://iam-apiserver/.well-known/jwks.json",
   })
   verifier, _ := authverifier.NewTokenVerifier(&sdk.TokenVerifyConfig{}, jwksManager, nil)
   result, err := verifier.Verify(ctx, accessToken, nil)
   ```

2. **身份查询**：使用 gRPC SDK
   ```go
   client := sdk.NewClient(grpcConn)
   user, err := client.Identity().GetByID(ctx, userID)
   ```

3. **监护关系判定**：gRPC 查询
   ```go
   guardians, err := client.Guardianship().ListByChild(ctx, childID)
   ```

#### 集成点

| 集成点 | 协议 | 说明 |
|-------|------|------|
| 用户认证 | REST / gRPC | 登录、刷新、验证 |
| 身份查询 | gRPC | 用户/儿童信息 |
| 权限检查 | gRPC / REST | PDP 调用 |
| JWKS | REST HTTP | 公钥获取 |
| 服务 Token | gRPC | 服务间认证 |

### 8.2 中间件集成

#### JWT 认证中间件

[internal/pkg/middleware/authn/jwt_middleware.go]

```go
// 创建中间件
authMiddleware := authnMiddleware.NewJWTAuthMiddleware(
    tokenService,  // Token 验证服务
    casbinAdapter, // 可选的授权引擎
)

// 使用中间件
engine.Use(authMiddleware.AuthRequired())
engine.Use(authMiddleware.RequireRole("admin"))
engine.Use(authMiddleware.RequirePermission("read", "document"))
```

#### 中间件功能

- `AuthRequired()`: JWT 令牌必需
- `RequireRole(roles...)`: 角色检查
- `RequirePermission(obj, act)`: 权限检查

### 8.3 gRPC ACL

**配置文件**：[configs/grpc_acl.yaml](configs/grpc_acl.yaml)

```yaml
services:
  # 认证服务 (authn)
  - name: AuthService
    methods:
      - Login
      - RefreshToken
      - VerifyToken
    roles: ["*"]  # 所有用户可访问
    
  # 授权服务 (authz)
  - name: AuthorizationService
    methods:
      - Check
    roles: ["*"]  # 所有用户可访问
    
  # 身份服务 (identity)
  - name: IdentityService
    methods:
      - GetUser
      - ListChildren
    roles: ["*"]  # 所有用户可访问
    
  # 管理接口 (admin)
  - name: AdminService
    methods:
      - "*"
    roles: ["admin"]  # 仅管理员可访问
```

### 8.4 SDK 包装

**SDK 位置**：[pkg/sdk/](pkg/sdk/)

```go
// 快速开始
client, _ := sdk.NewClient(
    sdk.WithGRPCTarget("iam-apiserver:50051"),
    sdk.WithJWKSURL("http://iam-apiserver/.well-known/jwks.json"),
)

// 用户认证
auth, _ := client.Auth().VerifyToken(ctx, token)

// 身份查询
identity, _ := client.Identity().GetByID(ctx, userID)

// 监护关系
guardians, _ := client.Guardianship().ListByChild(ctx, childID)

// 授权检查
allowed, _ := client.Authz().Check(ctx, subject, domain, object, action)
```

---

## 9. 当前关键特性

### 9.1 已实现的核心能力

| 特性 | 实现状态 | 说明 |
|------|--------|------|
| **多端支持** | ✅ | 微信小程序、Web、企业微信 |
| **多凭据** | ✅ | 密码、OTP、OAuth |
| **JWT + JWKS** | ✅ | 标准 JWT、公钥轮换 |
| **RBAC** | ✅ | 基于角色的权限控制 |
| **Casbin 引擎** | ✅ | 高性能策略执行 |
| **监护关系** | ✅ | 家长-儿童绑定、验证 |
| **HTTP/REST API** | ✅ | OpenAPI 契约 |
| **gRPC API** | ✅ | Protobuf 服务定义 |
| **缓存治理** | ✅ | 统一缓存监控 |
| **联想搜索** | ✅ | 儿童快速查询 |
| **六边形架构** | ✅ | 适配器模式实践 |
| **CQRS 模式** | ✅ (authz) | 部分模块实现 |

### 9.2 部分落地的能力

| 特性 | 实现状态 | 说明 |
|------|--------|------|
| **完整 CQRS** | 🟡 | authz 清晰，uc 部分实现 |
| **事件驱动** | 🟡 | 可选消息队列集成 |
| **分布式追踪** | 🟡 | Prometheus/Observability 支持 |
| **API 版本控制** | 🟡 | 已支持 v1，扩展机制就位 |

### 9.3 未来扩展方向

- [ ] 批量权限判定 API
- [ ] Explain 能力（解释权限决策）
- [ ] 审计日志完整链
- [ ] 批量用户管理
- [ ] SSO 集成

---

## 10. 总结

### 10.1 架构亮点

1. **清晰的六边形架构**：interface/application/domain/infra 四层分工明确
2. **强类型系统**：Go 接口驱动，易于测试和扩展
3. **CQRS 在授权域**：authz 模块展示了高质量的读写分离实践
4. **模块装配明确**：Container 中集中式依赖注入
5. **缓存治理统一**：通过 FamilyInspector 模式管理多个缓存族

### 10.2 代码质量

- 仓储接口明确，易于 Mock 和单元测试
- 领域对象富含业务逻辑，不只是数据容器
- 应用服务专注编排，保持轻量
- 基础设施层隐藏技术细节

### 10.3 集成友好

- REST + gRPC 双协议支持
- JWKS 端点支持本地验签
- SDK 包装简化接入
- 中间件支持认证和授权

### 10.4 文档体系完善

- [docs/00-概览/](docs/00-概览/) - 架构概览
- [docs/02-业务域/](docs/02-业务域/) - 业务详解
- [docs/04-基础设施与运维/](docs/04-基础设施与运维/) - 运维指南
- [docs/05-专题分析/](docs/05-专题分析/) - 深度剖析

---

## 附录

### A. 关键接口定义

#### 认证仓储

```go
// domain/authn/authentication/repository.go
type CredentialRepository interface {
    FindPasswordCredential(ctx context.Context, accountID meta.ID) (credentialID meta.ID, passwordHash string, err error)
    FindPhoneOTPCredential(ctx context.Context, phoneE164 string) (accountID, userID, credentialID meta.ID, err error)
    FindOAuthCredential(ctx context.Context, idpType, appID, idpIdentifier string) (accountID, userID, credentialID meta.ID, err error)
}

type AccountRepository interface {
    FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*UsernameLoginLookup, error)
    GetAccountStatus(ctx context.Context, accountID meta.ID) (enabled, locked bool, err error)
}
```

#### 授权仓储

```go
// domain/authz/role/repository.go
type Repository interface {
    Create(ctx context.Context, role *Role) error
    FindByID(ctx context.Context, id meta.ID) (*Role, error)
    Update(ctx context.Context, role *Role) error
    Delete(ctx context.Context, id meta.ID) error
    FindByFilter(ctx context.Context, filter ListFilter) ([]*Role, error)
}
```

#### Casbin 适配器

```go
// domain/authz/policy/interfaces.go
type CasbinAdapter interface {
    AddPolicy(ctx context.Context, rules ...PolicyRule) error
    RemovePolicy(ctx context.Context, rules ...PolicyRule) error
    AddGroupingPolicy(ctx context.Context, rules ...GroupingRule) error
    RemoveGroupingPolicy(ctx context.Context, rules ...GroupingRule) error
    Enforce(ctx context.Context, sub, dom, obj, act string) (bool, error)
}
```

### B. 重要文件链接表

| 文件 | 链接 |
|------|------|
| 项目 README | [README.md](README.md) |
| 架构总览 | [docs/00-概览/01-系统架构总览.md](docs/00-概览/01-系统架构总览.md) |
| 六边形架构实践 | [docs/04-基础设施与运维/01-六边形架构实践.md](docs/04-基础设施与运维/01-六边形架构实践.md) |
| CQRS 模式实践 | [docs/04-基础设施与运维/02-CQRS模式实践.md](docs/04-基础设施与运维/02-CQRS模式实践.md) |
| REST 契约 | [api/rest/](api/rest/) |
| gRPC 契约 | [api/grpc/iam/](api/grpc/iam/) |
| SDK 文档 | [pkg/sdk/docs/](pkg/sdk/docs/) |

---

**报告完成时间**: 2026年4月21日  
**探索深度**: Thorough (全面深入)  
**覆盖范围**: 架构 / 业务域 / 技术栈 / 设计模式 / 集成边界
