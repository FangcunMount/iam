# IAM 源码导航与对比清单

## 概述

本文档按模块、按层级组织所有关键源代码文件，便于快速定位实现细节，并与文档进行对标对比。

---

## 1. 架构文档与源码对应

### 1.1 六边形架构

| 文档 | 位置 | 源码体现 |
|------|------|---------|
| [04-基础设施与运维/01-六边形架构实践.md](docs/04-基础设施与运维/01-六边形架构实践.md) | 架构概念 | 见 1.2-1.5 |

### 1.2 Interface Layer (驱动适配器)

#### REST Adapters

```
internal/apiserver/interface/
├── authn/
│   └── restful/
│       ├── router.go                 # 认证路由注册
│       ├── handler/
│       │   ├── auth.go              # 登录/注册处理
│       │   ├── token.go             # Token 刷新/验证
│       │   ├── account.go           # 账户管理
│       │   ├── jwks.go              # JWKS 端点
│       │   ├── session_admin.go     # 会话管理
│       │   ├── base.go              # 基础处理器
│       │   ├── request/             # 请求 DTO
│       │   └── response/            # 响应 DTO
│       └── middleware/              # HTTP 中间件
├── authz/
│   └── restful/
│       ├── handler/
│       │   ├── role.go              # 角色管理 CRUD
│       │   ├── policy.go            # 策略管理
│       │   ├── resource.go          # 资源管理
│       │   ├── assignment.go        # 分配管理
│       │   └── check.go             # 权限检查
│       └── router.go
├── uc/
│   ├── restful/
│   │   ├── handler/
│   │   │   ├── identity.go          # 身份端点
│   │   │   ├── guardianship.go      # 监护关系端点
│   │   │   └── child.go             # 儿童管理端点
│   │   └── router.go
│   └── grpc/
│       ├── identity/
│       │   └── service.go           # 身份 gRPC 服务
│       ├── guardianship/
│       │   └── service.go           # 监护关系 gRPC 服务
│       └── lifecycle/
│           └── service.go           # 生命周期 gRPC 服务
├── idp/
│   └── restful/
│       ├── handler/
│       │   ├── wechat_app.go        # 微信应用管理
│       │   └── tenant_idp.go        # 租户 IDP 配置
│       └── router.go
└── suggest/
    └── restful/
        ├── handler/
        │   └── search.go            # 联想搜索端点
        └── router.go
```

#### gRPC Adapters

```
internal/apiserver/interface/authn/grpc/
├── service.go                       # AuthService gRPC 实现
│   ├── Login()
│   ├── RefreshToken()
│   ├── RevokeAccessToken()
│   ├── RevokeRefreshToken()
│   ├── VerifyToken()
│   └── IssueServiceToken()
└── jwks_service.go                  # JWKSService gRPC 实现
    ├── GetJWKS()
    └── RotateKey()

internal/apiserver/interface/authz/grpc/
└── service.go                       # AuthorizationService gRPC 实现
    ├── Check()
    ├── CheckBatch()
    └── Explain()
```

#### 中间件

```
internal/pkg/middleware/
├── authn/
│   └── jwt_middleware.go            # JWT 认证中间件
│       ├── AuthRequired()
│       ├── RequireRole()
│       └── RequirePermission()
└── cors/
    └── cors.go                      # CORS 中间件
```

### 1.3 Application Layer (应用服务)

#### 认证应用服务

```
internal/apiserver/application/authn/
├── login/
│   ├── services.go                  # LoginApplicationService 接口
│   └── services_impl.go             # 登录实现 (密码/OTP/OAuth)
├── token/
│   ├── services.go                  # TokenApplicationService 接口
│   └── services_impl.go             # Token 签发/刷新/验证
├── session/
│   ├── services.go                  # SessionApplicationService 接口
│   └── services_impl.go             # Session 生命周期
├── account/
│   ├── services.go                  # AccountApplicationService 接口
│   └── services_impl.go             # 账户管理
├── register/
│   ├── services.go                  # RegisterApplicationService 接口
│   └── services_impl.go             # 用户注册流程
├── loginprep/
│   ├── services.go                  # LoginPreparationService 接口
│   ├── services_impl.go
│   ├── login_otp.go                 # OTP 登录准备
│   └── login_otp_test.go
├── jwks/
│   ├── services.go                  # JWKS 应用服务
│   ├── key_management.go            # 密钥管理
│   ├── key_publish.go               # 密钥发布
│   ├── key_rotation.go              # 密钥轮换
│   ├── key_management_test.go
│   └── key_rotation_test.go
└── uow/
    └── uow.go                       # Unit of Work (事务边界)
```

#### 授权应用服务

```
internal/apiserver/application/authz/
├── role/
│   ├── command_service.go           # RoleCommandService (创建/更新/删除)
│   ├── query_service.go             # RoleQueryService (查询)
│   ├── services_impl.go
│   └── services_impl_test.go
├── resource/
│   ├── command_service.go           # ResourceCommandService
│   ├── query_service.go             # ResourceQueryService
│   └── services_impl.go
├── policy/
│   ├── command_service.go           # PolicyCommandService (含 Casbin 同步)
│   ├── query_service.go             # PolicyQueryService
│   ├── services_impl.go
│   └── services_impl_test.go
├── assignment/
│   ├── command_service.go           # AssignmentCommandService (含 Casbin g 规则)
│   ├── query_service.go             # AssignmentQueryService
│   ├── services_impl.go
│   └── services_impl_test.go
└── check/
    ├── service.go                   # CheckService (PDP)
    └── services_impl.go
```

#### 用户域应用服务

```
internal/apiserver/application/uc/
├── user/
│   ├── services.go                  # 用户应用服务接口
│   ├── services_impl.go             # 实现 (Register/Rename/UpdateContact)
│   └── services_impl_test.go
├── child/
│   ├── services.go                  # 儿童应用服务接口
│   ├── services_impl.go             # 实现 (Register/Update/Query)
│   └── services_impl_test.go
├── guardianship/
│   ├── services.go                  # 监护关系应用服务接口
│   ├── services_impl.go             # 实现 (Create/Revoke/Query)
│   └── services_impl_test.go
├── uow/
│   └── uow.go                       # Unit of Work (用户域事务)
└── dto.go                           # 数据传输对象
```

#### IDP 应用服务

```
internal/apiserver/application/idp/
├── wechat/
│   ├── services.go
│   └── services_impl.go
└── tenant_idp/
    ├── services.go
    └── services_impl.go
```

#### Suggest 应用服务

```
internal/apiserver/application/suggest/
├── services.go                      # 联想搜索服务
└── services_impl.go
```

#### 缓存治理

```
internal/apiserver/application/cachegovernance/
├── service.go                       # ReadService (缓存监控)
├── service_test.go
├── model.go                         # 数据模型 (FamilyView/Overview)
├── jwks_inspector.go                # JWKS 缓存监控
├── token_inspector.go               # Token 缓存监控
└── session_inspector.go             # Session 缓存监控
```

### 1.4 Domain Layer (领域模型)

#### 认证领域

```
internal/apiserver/domain/authn/
├── account/
│   ├── account.go                   # Account 聚合根
│   ├── creator.go                   # AccountCreator 工厂
│   └── repository.go                # AccountRepository 接口
├── authentication/
│   ├── authenticater.go             # Authenticater 域服务 (多策略认证)
│   ├── authentication.go            # 认证相关值对象
│   ├── auth-password.go             # 密码认证实现
│   ├── auth-otp.go                  # OTP 认证实现
│   ├── auth-wechat-com.go           # WeChat 认证实现
│   ├── auth-wecom.go                # 企业微信认证实现
│   ├── external.go                  # IdentityProvider 接口
│   ├── external_test.go
│   └── repository.go                # CredentialRepository & AccountRepository 接口
├── credential/
│   ├── credential.go                # Credential 值对象
│   └── repository.go                # CredentialRepository 接口
├── session/
│   ├── session.go                   # Session 聚合根
│   ├── manager.go                   # SessionManager 域服务
│   └── repository.go                # SessionRepository 接口
├── token/
│   ├── token.go                     # Token 值对象 + TokenPair
│   ├── token_claims.go              # TokenClaims (JWT payload)
│   ├── issuer.go                    # TokenIssuer 域服务 (签发/刷新)
│   ├── refresher.go                 # TokenRefresher 域服务
│   ├── verifier.go                  # TokenVerifier 域服务
│   └── repository.go                # TokenRepository 接口
└── jwks/
    ├── jwks.go                      # JWKS & Key 值对象
    ├── key_set.go
    └── repository.go                # JWKSRepository 接口
```

#### 授权领域

```
internal/apiserver/domain/authz/
├── role/
│   ├── role.go                      # Role 聚合根
│   ├── interfaces.go                # 角色相关接口
│   ├── validator.go                 # RoleValidator (业务规则)
│   ├── repository.go                # RoleRepository 接口
│   ├── role_test.go
│   ├── role_validator_test.go
│   └── commands.go                  # 角色命令对象 (CreateRoleCommand 等)
├── resource/
│   ├── resource.go                  # Resource 聚合根
│   ├── action.go                    # Action 值对象
│   ├── validator.go                 # ResourceValidator
│   ├── repository.go                # ResourceRepository 接口
│   ├── commands.go                  # 资源命令对象
│   └── resource_test.go
├── policy/
│   ├── policy.go                    # PolicyRule 聚合根
│   ├── policy_version.go            # PolicyVersion 值对象
│   ├── casbin_adapter.go            # CasbinAdapter 接口 (重要!)
│   ├── repository.go                # PolicyRepository 接口
│   ├── commands.go                  # 策略命令对象
│   ├── policy_test.go
│   └── event.go                     # 版本通知事件 (可选)
├── assignment/
│   ├── assignment.go                # Assignment 聚合根
│   ├── validator.go                 # AssignmentValidator
│   ├── repository.go                # AssignmentRepository 接口
│   ├── commands.go                  # 分配命令对象
│   └── assignment_test.go
└── tenants/
    ├── tenant.go                    # Tenant 上下文
    └── repository.go                # TenantRepository 接口
```

#### 用户域

```
internal/apiserver/domain/uc/
├── user/
│   ├── user.go                      # User 聚合根
│   ├── status.go                    # UserStatus 值对象 (枚举)
│   ├── repository.go                # UserRepository 接口
│   └── commands.go                  # 用户命令对象
├── child/
│   ├── child.go                     # Child 聚合根
│   ├── gender.go                    # Gender 值对象
│   ├── repository.go                # ChildRepository 接口
│   └── commands.go                  # 儿童命令对象
└── guardianship/
    ├── guardianship.go              # Guardianship 聚合根 (关系对象)
    ├── relation_type.go             # RelationType 值对象
    ├── repository.go                # GuardianshipRepository 接口
    ├── commands.go                  # 监护关系命令对象
    └── errors.go                    # 域错误
```

#### IDP 域

```
internal/apiserver/domain/idp/
├── wechatapp/
│   ├── wechat_app.go                # WechatApp 聚合根 (应用配置)
│   ├── access_token.go              # AppAccessToken 值对象
│   ├── repository.go                # WechatAppRepository 接口
│   ├── access_token_cache.go        # AccessTokenCache 接口
│   ├── commands.go                  # 配置命令对象
│   └── errors.go
└── tenant_idp/
    ├── tenant_idp.go                # TenantIDP 聚合根 (租户配置)
    ├── repository.go                # TenantIDPRepository 接口
    └── commands.go
```

#### Suggest 域

```
internal/apiserver/domain/suggest/
├── suggest.go                       # Suggest 相关接口
├── model.go                         # 数据模型
└── query.go                         # 查询对象
```

### 1.5 Infrastructure Layer (基础设施)

#### MySQL 仓储实现

```
internal/apiserver/infra/mysql/
├── account/
│   ├── po.go                        # PersistentObject (数据库映射)
│   ├── mapper.go                    # Domain Model ↔ PO 映射
│   └── repository.go                # Repository 实现
├── credential/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── user/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── child/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── guardianship/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── role/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── resource/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── policy/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── assignment/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── session/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── jwks/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
├── wechatapp/
│   ├── po.go
│   ├── mapper.go
│   └── repository.go
└── suggest/
    ├── po.go
    ├── loader.go                    # 数据加载器
    └── repository.go
```

#### Redis 缓存实现

```
internal/apiserver/infra/redis/
├── session_store.go                 # Session 缓存实现
├── token_store.go                   # Token 缓存实现
├── otp_store.go                     # OTP 验证码缓存
├── otp_verifier_impl.go
├── accesstoken_cache.go             # WeChat AccessToken 缓存
├── cache_inspector.go               # 缓存监控支持
├── redis_utils.go                   # 工具函数 (键管理、序列化)
└── client_utils.go
```

#### Casbin 适配器

```
internal/apiserver/infra/casbin/
├── adapter.go                       # CasbinAdapter 实现 (核心!)
│   ├── NewCasbinAdapter()
│   ├── AddPolicy()
│   ├── RemovePolicy()
│   ├── AddGroupingPolicy()
│   ├── RemoveGroupingPolicy()
│   └── Enforce()
├── enforcer_wrapper.go              # Enforcer 包装器
└── rule_converter.go                # 规则转换工具
```

#### JWT 实现

```
internal/apiserver/infra/jwt/
├── generator.go                     # JWT 生成器
│   ├── GenerateAccessToken()
│   ├── GenerateRefreshToken()
│   └── GenerateServiceToken()
├── verifier.go                      # JWT 验证器
│   ├── VerifyAccessToken()
│   ├── VerifyRefreshToken()
│   └── VerifyServiceToken()
├── claims.go                        # JWT Claims 结构
└── keystore.go                      # 密钥存储
```

#### 加密实现

```
internal/apiserver/infra/crypto/
├── password_hasher.go               # 密码哈希 (PHC 格式)
│   ├── HashPassword()
│   └── VerifyPassword()
├── aes_encryptor.go                 # AES 加密
├── rsa_encryptor.go                 # RSA 加密 (可选)
└── hmac_signer.go                   # HMAC 签名
```

#### 认证策略实现

```
internal/apiserver/infra/authentication/
├── password_authenticater.go        # 密码认证
├── otp_authenticater.go             # OTP 认证
├── wechat_authenticater.go          # WeChat 认证
├── wecom_authenticater.go           # 企业微信认证
├── oauth_handler.go                 # OAuth 通用处理
└── strategy_factory.go              # 策略工厂
```

#### WeChat 集成

```
internal/apiserver/infra/wechat/
├── client.go                        # WeChat API 客户端
├── code2session.go                  # Code2Session 实现
├── phone_verifier.go                # 手机号验证
└── utils.go

internal/apiserver/infra/wechatapi/
├── wechat_impl.go                   # WeChat SDK 实现
└── types.go
```

#### 其他基础设施

```
internal/apiserver/infra/
├── cache/
│   ├── family.go                    # 缓存族定义
│   ├── inspector.go                 # 缓存监控接口
│   └── constants.go                 # 常量定义
├── messaging/
│   ├── publisher.go                 # 消息发布
│   └── event_types.go               # 事件类型
├── scheduler/
│   ├── scheduler.go                 # 定时任务调度
│   └── job_factory.go               # 任务工厂
├── sms/
│   ├── provider.go                  # SMS 提供商接口
│   └── implementations.go            # SMS 实现
└── logger/
    ├── logger.go                    # 日志记录
    └── fields.go                    # 日志字段
```

#### 联想搜索基础设施

```
internal/apiserver/infra/suggest/
├── search/
│   ├── store.go                     # 内存搜索存储 (Trie + Hash)
│   ├── trie.go                      # Trie 树实现
│   ├── hash_map.go                  # Hash 映射
│   ├── term.go                      # 搜索项数据结构
│   └── term_test.go
├── index/
│   ├── builder.go                   # 索引构建器
│   └── updater.go                   # 索引更新器
└── snapshot/
    ├── saver.go                     # 快照保存
    └── loader.go                    # 快照加载
```

### 1.6 模块装配 (Container)

```
internal/apiserver/container/
├── container.go                     # 主容器 (所有模块)
├── assembler/
│   ├── authn.go                     # 认证模块装配
│   ├── authz.go                     # 授权模块装配
│   ├── uc.go                        # 用户域模块装配
│   ├── idp.go                       # IDP 模块装配
│   ├── suggest.go                   # 联想搜索模块装配
│   ├── cachegovernance.go           # 缓存治理装配
│   └── external_services.go         # 外部服务装配
└── options.go
```

### 1.7 核心入口和启动

```
internal/apiserver/
├── app.go                           # App 实例创建
├── routers.go                       # 所有路由注册
│   ├── registerBaseRoutes()
│   ├── registerAuthnRoutes()
│   ├── registerAuthzRoutes()
│   ├── registerUCRoutes()
│   ├── registerIDPRoutes()
│   └── registerSuggestRoutes()
├── server.go                        # 服务器实现
│   ├── createAPIServer()
│   ├── buildGenericServer()
│   ├── buildGRPCServer()
│   └── Run() / RunE()
├── run.go                           # 运行逻辑
├── database.go                      # 数据库连接管理
├── database_test.go
├── testhelpers/                     # 测试辅助工具
└── options/
    └── options.go                   # 启动选项
```

#### 进程入口

```
cmd/apiserver/
└── apiserver.go                     # main() 入口点
```

---

## 2. API 契约与文档

### 2.1 REST 契约

```
api/rest/
├── authn.v1.yaml                    # 认证 REST API
│   ├── POST /api/v1/authn/login
│   ├── POST /api/v1/authn/refresh_token
│   ├── POST /api/v1/authn/logout
│   ├── POST /api/v1/authn/verify
│   ├── POST /api/v1/authn/login/prep/phone-otp
│   └── GET /.well-known/jwks.json
├── authz.v1.yaml                    # 授权 REST API
│   ├── POST /api/v1/authz/check
│   ├── CRUD /api/v1/authz/roles/*
│   ├── CRUD /api/v1/authz/resources/*
│   ├── CRUD /api/v1/authz/policies/*
│   └── CRUD /api/v1/authz/assignments/*
├── identity.v1.yaml                 # 身份 REST API
│   ├── GET /api/v1/identity/me
│   ├── GET /api/v1/identity/me/children
│   ├── POST /api/v1/identity/children/register
│   └── POST /api/v1/identity/guardians/grant
├── idp.v1.yaml                      # IDP REST API
│   ├── CRUD /api/v1/idp/wechat-apps/*
│   └── CRUD /api/v1/idp/tenant-config/*
└── suggest.v1.yaml                  # 联想搜索 API
    └── GET /api/v1/suggest/child?k=...
```

### 2.2 gRPC 契约

```
api/grpc/iam/
├── authn/v1/
│   ├── authn.proto                  # 认证服务
│   │   ├── service AuthService {
│   │   │   ├── rpc Login(LoginRequest) returns (TokenPair);
│   │   │   ├── rpc RefreshToken(RefreshRequest) returns (TokenPair);
│   │   │   ├── rpc VerifyToken(VerifyRequest) returns (TokenVerifyResp);
│   │   │   ├── rpc IssueServiceToken(...);
│   │   │   └── rpc Revoke*(...)
│   │   ├── message LoginRequest
│   │   ├── message TokenPair
│   │   └── ...
│   └── jwks.proto                   # JWKS 服务
│       ├── service JWKSService {
│       │   ├── rpc GetJWKS(...);
│       │   └── rpc RotateKey(...)
│       └── message JWKSResponse
├── authz/v1/
│   └── authz.proto                  # 授权服务
│       ├── service AuthorizationService {
│       │   ├── rpc Check(CheckRequest) returns (CheckResponse);
│       │   ├── rpc CheckBatch(...);
│       │   └── rpc Explain(...)
│       └── message CheckRequest
├── identity/v1/
│   ├── identity.proto               # 身份服务
│   ├── guardianship.proto           # 监护关系服务
│   └── lifecycle.proto              # 生命周期服务
└── idp/v1/
    └── idp.proto                    # IDP 服务
```

---

## 3. 配置与运行

### 3.1 配置文件

```
configs/
├── apiserver.dev.yaml               # 开发配置
├── apiserver.prod.yaml              # 生产配置
├── casbin_model.conf                # Casbin 授权模型
├── grpc_acl.yaml                    # gRPC ACL 规则
├── mysql/
│   └── schema.sql                   # 数据库 schema
│       ├── users
│       ├── auth_accounts
│       ├── auth_credentials
│       ├── children
│       ├── guardianships
│       ├── authz_roles
│       ├── authz_resources
│       ├── authz_assignments
│       ├── authz_policy_versions
│       ├── casbin_rule
│       └── jwks_keys
├── env/
│   ├── config.dev.env
│   └── config.prod.env
├── keys/
│   ├── server.crt
│   ├── server.key
│   ├── ca.crt
│   └── ...
└── nginx/
    └── conf.d/
        └── iam.conf                 # Nginx 配置
```

### 3.2 启动脚本

```
scripts/
├── dev.sh                           # 本地开发启动
├── quick-start-dev.sh               # 快速启动
├── test-dev-config.sh               # 测试配置
├── check-openapi-contracts.py       # OpenAPI 验证
├── check-route-contracts.py         # 路由验证
├── validate-openapi.sh              # OpenAPI 校验脚本
├── reset-openapi-from-swagger.py    # Swagger 重置
├── update-deps.sh                   # 依赖更新
├── proto/                           # Proto 编译脚本
└── cert/                            # 证书生成脚本
```

---

## 4. 文档导航

### 4.1 架构文档

```
docs/
├── 00-概览/
│   ├── 01-系统架构总览.md           # ★★★ 从这里开始
│   ├── 02-核心概念术语.md
│   └── 03-阅读路径&代码组织与事实来源.md
├── 01-运行时/
│   ├── 01-服务入口&HTTP与模块装配.md
│   ├── 02-gRPC与mTLS.md
│   ├── 03-HTTP认证中间件与身份上下文.md
│   └── 04-健康检查&debug路由与降级启动边界.md
├── 02-业务域/
│   ├── 01-authn-认证&Token&JWKS.md
│   ├── 02-authz-角色&策略&资源&Assignment.md
│   ├── 03-user-用户&儿童&Guardianship.md
│   └── 04-suggest-儿童联想搜索.md
├── 03-接口与集成/
│   ├── 01-REST契约与接入.md
│   ├── 02-gRPC契约与接入.md
│   ├── 03-授权接入与边界.md
│   ├── 04-身份接入与监护关系边界.md
│   ├── 05-QS接入IAM.md             # ★ QS 集成指南
│   └── 06-IAM-QS竖切边界-Token与授权快照.md
├── 04-基础设施与运维/
│   ├── 01-六边形架构实践.md         # ★ 架构实践
│   ├── 02-CQRS模式实践.md           # ★ CQRS 实践
│   ├── 03-命令&契约校验与开发流程.md
│   ├── 04-端口&证书与数据库迁移.md
│   └── 05-Seeddata与Collection集成补充.md
└── 05-专题分析/
    ├── 01-认证链路--从登录请求到Token与JWKS.md
    ├── 02-IAM认证语义拆层--用户状态&会话&Token边界.md
    ├── 03-授权判定链路--角色&策略&资源&Assignment&Casbin.md
    ├── 04-监护关系链路--用户&儿童&Guardianship的协作.md
    ├── 05-IAM缓存层--缓存层的设计与治理.md
    ├── 06-IAM缓存层--数据结构选择与Redis建模判断.md
    └── 07-SDK封装与接入价值.md
```

### 4.2 SDK 文档

```
pkg/sdk/
├── README.md                        # SDK 概览
├── docs/
│   ├── 01-quick-start.md            # 快速开始
│   ├── 02-grpc-client-setup.md      # gRPC 客户端
│   ├── 03-sdk-config.md             # SDK 配置
│   ├── 04-jwt-verification.md       # JWT 验证
│   ├── 05-service-auth.md           # 服务认证
│   └── examples/
│       ├── verifier/
│       │   └── main.go              # JWT 本地验证示例
│       ├── identity/
│       │   └── main.go              # 身份查询示例
│       └── authz/
│           └── main.go              # 权限检查示例
└── _examples/
    └── 各个示例项目
```

---

## 5. 关键接口与类型定义

### 5.1 登录流程关键接口

```
认证决策链：Request → Handler → LoginService → Authenticater → Principal → Session → TokenPair

关键类型定义文件：
- domain/authn/authentication/repository.go     (CredentialRepository, AccountRepository)
- domain/authn/authentication/authenticater.go  (Authenticater, AuthDecision)
- domain/authn/account/account.go              (Account 聚合根)
- domain/authn/session/session.go              (Session 聚合根)
- domain/authn/token/token.go                  (Token, TokenPair)
- application/authn/login/services.go          (LoginApplicationService)
```

### 5.2 权限判定关键接口

```
授权决策链：Request → Handler → CheckService → CasbinAdapter → Enforce

关键类型定义文件：
- domain/authz/policy/interfaces.go            (CasbinAdapter)
- domain/authz/role/role.go                    (Role 聚合根)
- domain/authz/assignment/assignment.go        (Assignment)
- infra/casbin/adapter.go                      (CasbinAdapter 实现)
- application/authz/role/command_service.go    (RoleCommandService)
- application/authz/role/query_service.go      (RoleQueryService)
```

### 5.3 用户管理关键接口

```
用户创建链：Request → Handler → UserService → UserRepository → User 聚合根

关键类型定义文件：
- domain/uc/user/user.go                       (User 聚合根)
- domain/uc/child/child.go                     (Child 聚合根)
- domain/uc/guardianship/guardianship.go       (Guardianship 关系)
- application/uc/uow/uow.go                    (UnitOfWork 事务)
- application/uc/user/services.go              (用户应用服务)
```

---

## 6. 代码导航速查表

### 6.1 我想要理解 XXX

| 想要理解的内容 | 从这里开始 |
|---------------|-----------|
| 整体架构 | [docs/00-概览/01-系统架构总览.md](docs/00-概览/01-系统架构总览.md) |
| 登录流程 | [docs/05-专题分析/01-认证链路.md](docs/05-专题分析/01-认证链路.md) + [internal/apiserver/interface/authn/restful/handler/auth.go](internal/apiserver/interface/authn/restful/handler/auth.go) |
| 权限检查 | [docs/05-专题分析/03-授权判定链路.md](docs/05-专题分析/03-授权判定链路.md) + [internal/apiserver/infra/casbin/adapter.go](internal/apiserver/infra/casbin/adapter.go) |
| 监护关系 | [docs/02-业务域/03-user-用户&儿童&Guardianship.md](docs/02-业务域/03-user-用户&儿童&Guardianship.md) + [internal/apiserver/domain/uc/guardianship/guardianship.go](internal/apiserver/domain/uc/guardianship/guardianship.go) |
| CQRS 实践 | [docs/04-基础设施与运维/02-CQRS模式实践.md](docs/04-基础设施与运维/02-CQRS模式实践.md) + [internal/apiserver/application/authz/role/](internal/apiserver/application/authz/role/) |
| 六边形架构 | [docs/04-基础设施与运维/01-六边形架构实践.md](docs/04-基础设施与运维/01-六边形架构实践.md) + [internal/apiserver/container/assembler/](internal/apiserver/container/assembler/) |
| 缓存系统 | [docs/05-专题分析/05-IAM缓存层.md](docs/05-专题分析/05-IAM缓存层.md) + [internal/apiserver/infra/redis/](internal/apiserver/infra/redis/) |
| SDK 集成 | [docs/05-专题分析/07-SDK封装与接入价值.md](docs/05-专题分析/07-SDK封装与接入价值.md) + [pkg/sdk/docs/01-quick-start.md](pkg/sdk/docs/01-quick-start.md) |
| QS 接入 | [docs/03-接口与集成/05-QS接入IAM.md](docs/03-接口与集成/05-QS接入IAM.md) |

### 6.2 我想要改进 XXX

| 想要改进的内容 | 涉及的代码文件 |
|---------------|--------------|
| 添加新认证策略 | [internal/apiserver/infra/authentication/](internal/apiserver/infra/authentication/) + [internal/apiserver/domain/authn/authentication/](internal/apiserver/domain/authn/authentication/) |
| 添加新角色类型 | [internal/apiserver/domain/authz/role/](internal/apiserver/domain/authz/role/) + [internal/apiserver/application/authz/role/](internal/apiserver/application/authz/role/) |
| 添加新资源 | [internal/apiserver/domain/authz/resource/](internal/apiserver/domain/authz/resource/) + [internal/apiserver/interface/authz/restful/handler/resource.go](internal/apiserver/interface/authz/restful/handler/resource.go) |
| 优化缓存 | [internal/apiserver/infra/redis/](internal/apiserver/infra/redis/) + [internal/apiserver/application/cachegovernance/](internal/apiserver/application/cachegovernance/) |
| 扩展联想搜索 | [internal/apiserver/infra/suggest/search/](internal/apiserver/infra/suggest/search/) + [internal/apiserver/application/suggest/](internal/apiserver/application/suggest/) |
| 添加新 IDP | [internal/apiserver/domain/idp/](internal/apiserver/domain/idp/) + [internal/apiserver/infra/wechat/](internal/apiserver/infra/wechat/) |
| 添加 API 端点 | [internal/apiserver/interface/*/restful/handler/](internal/apiserver/interface/*/restful/handler/) |

### 6.3 快速查找代码位置

| 代码特征 | 搜索关键字 | 代码位置 |
|---------|---------|--------|
| Repository 接口定义 | `type.*Repository interface` | `domain/*/repository.go` |
| 聚合根定义 | `type.*struct` (在 domain 里带多个方法) | `domain/*/**.go` |
| 应用服务 | `type.*ApplicationService interface` | `application/*/*.go` |
| Handler | `func (h *XXHandler) YYY(c *gin.Context)` | `interface/*/restful/handler/` |
| gRPC 服务 | `func (s *Service) XXX(ctx context.Context,...)` | `interface/*/grpc/service.go` |
| 中间件 | `func XXXMiddleware(...) gin.HandlerFunc` | `internal/pkg/middleware/` |
| SQL 操作 | `db.Table(...).Where(...)` | `infra/mysql/*/repository.go` |
| Redis 操作 | `redis.NewClient(...).Set(...).Get(...)` | `infra/redis/*.go` |
| Casbin 规则 | `enforcer.AddPolicy(...)` | `infra/casbin/adapter.go` |

---

## 7. 测试代码位置

### 7.1 单元测试

```
// 由于篇幅限制，这里仅列出主要测试文件

internal/apiserver/
├── application/authn/login/services_impl_test.go
├── application/authn/token/services_impl_test.go
├── application/authn/register/services_impl_test.go
├── application/authz/role/services_impl_test.go
├── application/authz/policy/services_impl_test.go
├── application/authn/jwks/key_management_test.go
├── application/authn/jwks/key_rotation_test.go
├── application/cachegovernance/service_test.go
├── domain/authn/authentication/external_test.go
├── domain/authz/role/role_test.go
├── domain/authz/role/role_validator_test.go
├── infra/mysql/*/repository_test.go
├── interface/authn/restful/handler/*_test.go
└── interface/authn/grpc/*_test.go

cmd/apiserver/
├── apiserver_test.go
└── integration_test.go
```

### 7.2 集成测试

```
internal/apiserver/
├── routers_test.go                  # 路由测试
├── database_test.go                 # 数据库连接测试
└── server_test.go                   # 服务器启动测试
```

---

## 8. 构建与部署

### 8.1 构建命令

```
Makefile
├── build                 # 构建项目
├── build-apiserver      # 仅构建 apiserver
├── clean                # 清理构建产物
├── deps                 # 下载依赖
├── lint                 # 代码检查
├── fmt                  # 格式化
├── test                 # 运行测试
├── api-validate         # 验证 API 契约
└── proto                # 生成 Proto 代码
```

### 8.2 Docker 构建

```
build/docker/
├── Dockerfile                       # Docker 镜像构建
├── docker-compose.dev.yml           # 开发环境
├── docker-compose.prod.yml          # 生产环境
└── infra/                           # 基础设施配置
```

---

## 9. 常见问题快速定位

| 问题 | 相关代码位置 |
|------|-----------|
| 如何添加新的登录方式？ | [internal/apiserver/infra/authentication/](internal/apiserver/infra/authentication/) + [internal/apiserver/domain/authn/authentication/](internal/apiserver/domain/authn/authentication/) |
| 如何修改 Token 有效期？ | [internal/apiserver/domain/authn/token/](internal/apiserver/domain/authn/token/) + [internal/apiserver/application/authn/token/](internal/apiserver/application/authn/token/) |
| 如何添加新权限规则？ | [internal/apiserver/application/authz/policy/](internal/apiserver/application/authz/policy/) |
| 如何扩展用户信息？ | [internal/apiserver/domain/uc/user/](internal/apiserver/domain/uc/user/) + [configs/mysql/schema.sql](configs/mysql/schema.sql) |
| 如何使用 gRPC 接口？ | [api/grpc/iam/](api/grpc/iam/) + [internal/apiserver/interface/*/grpc/](internal/apiserver/interface/*/grpc/) |
| 如何配置 JWKS 轮换？ | [internal/apiserver/application/authn/jwks/key_rotation.go](internal/apiserver/application/authn/jwks/key_rotation.go) + [configs/apiserver.*.yaml](configs/apiserver.prod.yaml) |
| 如何集成新的 IDP？ | [internal/apiserver/domain/idp/](internal/apiserver/domain/idp/) + [internal/apiserver/infra/wechat/](internal/apiserver/infra/wechat/) |
| 如何改进缓存性能？ | [internal/apiserver/infra/redis/](internal/apiserver/infra/redis/) + [internal/apiserver/application/cachegovernance/](internal/apiserver/application/cachegovernance/) |

---

**最后更新**: 2026年4月21日
**代码版本**: Go 1.24.0
**主要框架**: Gin + gRPC + GORM + Casbin
