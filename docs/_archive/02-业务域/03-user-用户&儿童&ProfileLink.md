# User、Profile 与 ProfileLink

## 本文回答

本文回答：Identity/UC 域如何表达 User、Profile 和 ProfileLink，如何保证本人档案关系和 active 关系查询边界，为什么 ProfileLink 可承载监护关系语义但不是法律监护判定引擎，以及当前应用服务如何把当前用户视角和系统侧命令分开。

## 30 秒结论

- 当前标准关系模型是 `ProfileLink`，表示 User 与 Profile 之间的档案关系。
- `ProfileLink.Relation` 可表达 self、parent、grandparent、other 等业务语义；`Type` 区分 self 边和普通关系边。
- User、Profile、ProfileLink 都有各自领域模型和服务；跨对象写入由 application/identity 的 Unit of Work 保证事务边界。
- User 不天然拥有 self profile；用户主动创建本人档案时由 ProfileLink 建立逻辑保证最多一个 active self link，数据库迁移也增加了 active self link guard。
- ProfileLink 默认查询 active 关系；需要包含 revoked 的场景必须调用明确的 including-revoked 能力。
- ProfileLink 不是 AuthZ，也不是法律监护裁定；业务系统需要权限判定时应结合 AuthZ 或业务自己的规则。

## 主图：Identity/UC 领域模型

```mermaid
classDiagram
    class User {
      ID
      Name
      Phone
      Email
      Status
    }
    class Profile {
      ID
      Name
      Gender
      Birthday
    }
    class ProfileLink {
      ID
      User
      Profile
      Type
      Relation
      EstablishedAt
      RevokedAt
    }
    class Relation {
      self
      parent
      grandparent
      other
    }

    User "1" --> "*" ProfileLink
    Profile "1" --> "*" ProfileLink
    ProfileLink --> Relation
```

## 重点速查

| 关注点 | 当前答案 | 代码证据 |
| ---- | ---- | ---- |
| User 领域模型 | 用户资料、联系方式、状态。 | [../../internal/apiserver/domain/identity/user](../../internal/apiserver/domain/identity/user) |
| Profile 领域模型 | 儿童或个人档案字段。 | [../../internal/apiserver/domain/identity/profile](../../internal/apiserver/domain/identity/profile) |
| ProfileLink 领域模型 | User 与 Profile 的档案关系。 | [../../internal/apiserver/domain/identity/profilelink](../../internal/apiserver/domain/identity/profilelink) |
| User 应用服务 | 创建、编辑、状态变更、目录查询。 | [../../internal/apiserver/application/identity/user](../../internal/apiserver/application/identity/user) |
| Profile 应用服务 | 创建、编辑、当前用户 profile 访问、查询。 | [../../internal/apiserver/application/identity/profile](../../internal/apiserver/application/identity/profile) |
| ProfileLink 应用服务 | 系统侧命令、目录查询、当前用户视角关系访问。 | [../../internal/apiserver/application/identity/profilelink](../../internal/apiserver/application/identity/profilelink) |
| UoW | User/Profile/ProfileLink 事务仓库集合。 | [../../internal/apiserver/application/identity/uow/uow.go](../../internal/apiserver/application/identity/uow/uow.go) |
| 合同 | REST Identity 和 gRPC Identity/ProfileLink。 | [../../api/rest/identity.v2.yaml](../../api/rest/identity.v2.yaml)、[../../api/grpc/iam/identity/v2/identity.proto](../../api/grpc/iam/identity/v2/identity.proto) |

## 1. 模块边界

| 边界 | 本域负责 | 本域不负责 |
| ---- | ---- | ---- |
| 用户 | User 创建、查询、资料变更、状态变更。 | 登录凭据、session、token 签发。 |
| 档案 | Profile 创建、查询、资料变更、当前用户档案访问。 | 档案联想索引刷新；这是 Suggest。 |
| 关系 | ProfileLink 建立、撤销、查询、当前用户视角过滤。 | 角色/权限判定；这是 AuthZ。 |
| 自有档案 | 用户主动创建的 self profile 和 active self link 唯一性。 | 法律意义上的监护裁定；登录后自动创建 self profile。 |

## 2. 领域模型与不变量

```mermaid
flowchart TD
    User["User"]
    Profile["Profile"]
    Link["ProfileLink"]
    Self["TypeSelf + RelSelf"]
    Relation["TypeRelation + parent/grandparent/other"]
    Revoked["RevokedAt != nil"]
    Active["RevokedAt == nil"]

    User --> Link
    Profile --> Link
    Link --> Self
    Link --> Relation
    Link --> Active
    Link --> Revoked
```

| 模型 | 关键字段/行为 | 不变量 |
| ---- | ---- | ---- |
| `User` | name、phone、email、status。 | active user 才是可用用户；状态变化会影响 AuthN session。 |
| `Profile` | name、gender、birthday、id card。 | 创建和重命名时由实体方法校验名称。 |
| `ProfileLink` | user、profile、type、relation、established_at、revoked_at。 | active 关系由 `RevokedAt == nil` 判定；撤销只写 revoked time。 |
| `Relation` | self、parent、grandparent、other。 | self relation 对应 self type；普通关系不能被当成本人档案。 |

active self link guard 由数据库迁移 [../../internal/pkg/migration/migrations/000007_add_active_self_profile_link_guard.up.sql](../../internal/pkg/migration/migrations/000007_add_active_self_profile_link_guard.up.sql) 保护，防止一个用户同时拥有多个 active self profile link。

## 3. 领域服务

| 领域服务 | 解决的问题 | 代码入口 |
| ---- | ---- | ---- |
| `user.User` | 用户资料变更和状态变更实体行为。 | [../../internal/apiserver/domain/identity/user/user.go](../../internal/apiserver/domain/identity/user/user.go) |
| `user.UniquenessChecker` | 用户手机号唯一性检查。 | [../../internal/apiserver/domain/identity/user/uniqueness_checker.go](../../internal/apiserver/domain/identity/user/uniqueness_checker.go) |
| `profile.Profile` | 档案重命名、证件、基础资料实体行为。 | [../../internal/apiserver/domain/identity/profile/profile.go](../../internal/apiserver/domain/identity/profile/profile.go) |
| `ProfileLinker` | 建立/撤销档案关系，校验 user/profile 存在和 active 重复关系。 | [../../internal/apiserver/domain/identity/profilelink/linker.go](../../internal/apiserver/domain/identity/profilelink/linker.go) |
| `ProfileLinker` | 建立/撤销 ProfileLink，并在 self 关系建立时拒绝重复 active self link。 | [../../internal/apiserver/domain/identity/profilelink/linker.go](../../internal/apiserver/domain/identity/profilelink/linker.go) |

ProfileLinker 的边界很重要：它返回要持久化的领域对象，但不直接提交事务。提交由 application 层的 UoW 负责。

## 4. 应用服务

```mermaid
flowchart TD
    REST["REST identity handler"]
    GRPC["gRPC identity services"]
    UserApp["application/identity/user"]
    ProfileApp["application/identity/profile"]
    LinkApp["application/identity/profilelink"]
    UoW["application/identity/uow"]
    Domain["domain/identity"]
    DB["MySQL repos"]

    REST --> UserApp
    REST --> ProfileApp
    REST --> LinkApp
    GRPC --> UserApp
    GRPC --> ProfileApp
    GRPC --> LinkApp
    UserApp --> UoW
    ProfileApp --> UoW
    LinkApp --> UoW
    UoW --> Domain
    UoW --> DB
```

| 应用服务 | 调用者视角 | 职责 |
| ---- | ---- | ---- |
| `user.Creator` | 系统侧 | 创建 User。 |
| `user.Editor` | 系统侧/当前用户资料面 | 修改 User 资料。 |
| `user.StatusChanger` | 系统侧 | 激活、停用、封禁用户，并联动 session。 |
| `user.Directory` | 系统侧 | 按 ID/phone 查询用户。 |
| `profile.Creator` / `Editor` / `Directory` | 系统侧 | Profile 创建、修改和查询。 |
| `profile.MyProfiles` | 当前用户视角 | 创建/查看/修改当前用户可访问的 profile。 |
| `profilelink.Commands` | 系统侧 | 建立、撤销 ProfileLink。 |
| `profilelink.Directory` | 系统侧 | 查询 user/profile 关系，区分 active 与 including-revoked。 |
| `profilelink.MyProfileLinks` | 当前用户视角 | 当前用户 grant/list/revoke 自己相关的 ProfileLink。 |

## 5. 创建本人档案链路

```mermaid
sequenceDiagram
    participant Client as "REST/gRPC"
    participant MyProfiles as "profile.MyProfiles"
    participant UoW as "Identity UnitOfWork"
    participant Profile as "Profile domain"
    participant Repo as "Repositories"

    Client->>MyProfiles: "Create current user's profile"
    MyProfiles->>UoW: "WithinTx"
    UoW->>Profile: "NewProfile"
    UoW->>Repo: "Create profile"
    UoW->>Repo: "Check active self link when relation == self"
    UoW->>Repo: "Create ProfileLink"
    UoW-->>Client: "CreatedProfileResult"
```

这个流程解决的是“profile 创建成功但本人关系没建立”的一致性问题。当前应用层通过 UoW 把 profile 写入和 self ProfileLink 写入收在同一个事务边界内。

## 6. 建立与撤销 ProfileLink

```mermaid
sequenceDiagram
    participant Caller as "REST/gRPC"
    participant App as "profilelink.Commands"
    participant UoW as "Identity UnitOfWork"
    participant Linker as "ProfileLinker"
    participant Repo as "ProfileLink repo"

    Caller->>App: "Establish(user, profile, relation)"
    App->>UoW: "WithinTx"
    UoW->>Linker: "validate and create entity"
    Linker-->>UoW: "ProfileLink"
    UoW->>Repo: "Create"
    Caller->>App: "Revoke"
    App->>UoW: "WithinTx"
    UoW->>Linker: "Revoke"
    Linker-->>UoW: "ProfileLink with RevokedAt"
    UoW->>Repo: "Update"
```

查询默认行为：

| 查询能力 | 是否包含 revoked |
| ---- | ---- |
| `Get`、`ListProfilesForUser`、`ListLinksForProfile` | 否，只返回 active。 |
| `GetIncludingRevoked`、`ListProfilesForUserIncludingRevoked`、`ListLinksForProfileIncludingRevoked` | 是，调用名显式说明。 |
| `MyProfileLinks.List` | 根据当前用户视角和 DTO 过滤。 |

## 7. REST/gRPC 能力

| 接口面 | 当前能力 |
| ---- | ---- |
| REST `/api/v2/identity/me` | 当前用户资料读取和 patch。 |
| REST `/api/v2/identity/me/profiles` | 当前用户关联 profile。 |
| REST `/api/v2/identity/profiles` | profile 创建、查询、搜索和更新。 |
| REST `/api/v2/identity/profile-links` | ProfileLink 查询、建立和撤销。 |
| gRPC `IdentityRead` | 用户和 profile 读取。 |
| gRPC `ProfileLinkQuery` | HasProfileLink、ListProfiles、ListProfileLinks。 |
| gRPC `ProfileLinkCommand` | Establish、Revoke、BatchRevoke、Import。 |
| gRPC `IdentityLifecycle` | 用户生命周期。 |

运行时 protected routes 和 gRPC 注册在 [../01-运行时](../01-运行时/README.md) 展开。

## 8. 与 AuthN/AuthZ/Suggest 的关系

| 相邻模块 | 关系 |
| ---- | ---- |
| AuthN | AuthN account 通过 user id 关联 User；onboarding 会创建/复用 User 并保证 self ProfileLink。 |
| AuthZ | User 可作为授权 subject；ProfileLink 本身不是权限判定结果。 |
| Suggest | Suggest 从 profile 候选构建读模型，帮助搜索，不写 ProfileLink。 |
| IDP | 外部身份先经 IDP/AuthN 转为账号和用户，再进入 Identity 关系模型。 |

## 9. 设计模式

| 模式 | 为什么用 | IAM 中如何落地 | 代价和边界 |
| ---- | ---- | ---- | ---- |
| Aggregate/Entity | User、Profile、ProfileLink 有各自生命周期和不变量。 | domain/identity 下独立模型和 repository。 | 不强行把 Profile 聚合进 User，避免关系规则耦合。 |
| Domain Service | 建立关系需要同时验证 user/profile/link。 | `ProfileLinker`。 | 领域服务不提交事务。 |
| Unit of Work | 创建 profile 与 self link、建立关系和撤销关系需要原子提交。 | `application/identity/uow.UnitOfWork`。 | 应用层负责事务，不把事务泄漏到 domain。 |
| CQRS-lite | 当前用户视角、系统侧命令和目录查询变化原因不同。 | `Commands`、`Directory`、`MyProfileLinks`、`MyProfiles`。 | 接口变多，但权限边界更清晰。 |
| DTO/Mapper | REST/gRPC、应用结果和领域对象字段不同。 | `mapper.go`、DTO/result types。 | 映射层必须跟合同测试一起维护。 |
| Guarded Invariant | active self profile link 唯一性是强不变量。 | `ProfileLinker` + DB active self guard。 | 迁移和应用逻辑需共同维护。 |

## 10. 代码证据与验证

| 关注点 | 路径 |
| ---- | ---- |
| UC domain | [../../internal/apiserver/domain/identity](../../internal/apiserver/domain/identity) |
| UC application | [../../internal/apiserver/application/identity](../../internal/apiserver/application/identity) |
| REST Identity | [../../internal/apiserver/transport/rest/identity](../../internal/apiserver/transport/rest/identity) |
| gRPC Identity | [../../internal/apiserver/transport/grpc/service/identity](../../internal/apiserver/transport/grpc/service/identity) |
| REST contract | [../../api/rest/identity.v2.yaml](../../api/rest/identity.v2.yaml) |
| gRPC contract | [../../api/grpc/iam/identity/v2/identity.proto](../../api/grpc/iam/identity/v2/identity.proto) |

验证命令：

```bash
go test ./internal/apiserver/domain/identity/... ./internal/apiserver/application/identity/... ./internal/apiserver/transport/rest/identity/... ./internal/apiserver/transport/grpc/service/identity
```
