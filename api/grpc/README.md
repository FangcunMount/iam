# Identity gRPC API

IAM 仅负责统一身份（账号、儿童档案、监护关系等）；OA、运营、消息等内部系统通过 gRPC 访问这些领域能力。REST 接口仍面向“当前登录用户”，而 gRPC 仅对可信网络开放，凭借 mTLS/ServiceToken 鉴别调用方。

---

## 🎯 设计前置

1. **OA 独立**：员工、组织、部门由独立 OA 系统负责，IAM 不承载此类业务。
2. **多消费方**：
   - 运营后台：需要查询/批量编辑用户、儿童、监护关系。
   - 内部系统（消息、报表、风控等）：批量读取身份数据。
   - OA / 自动化流程：需要账号生命周期与监护关系写能力。
3. **契约驱动**：`api/grpc/iam/identity/v1/identity.proto` 是唯一合同，任何实现改动必须同步更新 proto 与 README。

---

## 📦 Proto 布局

```text
api/grpc/
└── iam/
    └── identity/
        └── v1/
            └── identity.proto
```

- Go import：`github.com/FangcunMount/iam/api/grpc/iam/identity/v1`
- Proto 包名：`iam.identity.v1`，新增字段只能追加，禁止复用 field number。

---

## 🧩 服务矩阵

| Service | 主要消费方 | 能力概览 |
| --------- | ------------ | ----------- |
| `IdentityRead` | 运营、OA、消息 | 获取/搜索用户与儿童 (`GetUser/BatchGetUsers/SearchUsers/GetChild/BatchGetChildren`) |
| `GuardianshipQuery` | 运营、消息 | 读取监护关系 (`IsGuardian/ListChildren/ListGuardians`) |
| `GuardianshipCommand` | OA、运营 | 写入监护关系 (`Add/Revoke/BatchRevoke/Import`) |
| `IdentityLifecycle` | OA、自动化 | 账号生命周期 (`Create/Update/Deactivate/Block`) |
| `AuthService` | 业务服务、网关 | 认证能力 (`VerifyToken/RefreshToken/RevokeToken/RevokeRefreshToken/IssueServiceToken`) |
| `JWKSService` | SDK、业务服务 | gRPC 方式获取 JWKS |

---

## 🔐 请求契约

- **ID**：全部使用十进制字符串（`user_id`, `child_id` 等），超过 `uint64` 返回 `INVALID_ARGUMENT`。
- **Metadata**：
  - `authorization: Bearer <service-token>`（或绑定 mTLS 证书）。
  - `x-request-id`：必填，用于日志/Tracing。
  - 写接口需传 `operator` 信息（可放在 metadata 或请求体 `OperatorContext`）。
- **超时**：P99 < 50 ms，客户端推荐超时 100~200 ms，可按业务策略重试。
- **分页**：`OffsetPagination.limit` 默认 20，最大 50；`offset` 默认 0。

---

## 🧾 核心消息语义

- **User**：包含状态、昵称、头像、联系方式（已脱敏展示）、外部账号列表、创建/更新时间。
- **Child**：儿童实名档案，含性别、出生日期、身高体重、证件脱敏号、时间戳。
- **Guardianship**：用户 ↔ 儿童 监护关系（关系类型、生效/撤销时间）。
- **ChildEdge**：`Child + Guardianship`，用于“用户监护的儿童”列表。
- **GuardianshipEdge**：`Guardianship + User`，用于“儿童的监护人”列表。
- **OperatorContext**：写接口必填，记录操作者、渠道、理由，服务端据此落审计日志。
完整字段定义见 `identity.proto`。

---

## 🛠️ 服务说明

### IdentityRead

- `GetUser / BatchGetUsers`：按用户 ID 查询账号，批量接口减少网络往返。
- `SearchUsers`：支持昵称关键字、手机号、邮箱等组合条件分页检索。
- `GetChild / BatchGetChildren`：查询儿童档案详情，用于运营、报表或消息系统。

### GuardianshipQuery

- `IsGuardian`：判定某用户是否监护指定儿童，若为真附带监护详情。
- `ListChildren`：列出用户监护的儿童（`ChildEdge`），支持分页。
- `ListGuardians`：列出儿童所有监护人（`GuardianshipEdge`）。

### GuardianshipCommand

- `AddGuardian`：创建监护关系，需要 `OperatorContext`。
- `RevokeGuardian / BatchRevokeGuardians`：撤销单条或批量关系，可通过 guardianship_id 或 (user_id, child_id) 指定。
- `ImportGuardians`：批量导入线下数据，支持部分成功。

### IdentityLifecycle

- `CreateUser`：创建账号（昵称/手机号/邮箱/外部身份等）。
- `UpdateUser`：更新账号基础资料。
- `DeactivateUser` / `BlockUser`：停用或封禁账号。

### 当前边界

- 当前运行时只注册 `IdentityRead`、`GuardianshipQuery`、`GuardianshipCommand`、`IdentityLifecycle`。
- 当前没有对外开放事件订阅型 gRPC，也不再保留未实现但可见的占位 RPC。

---

## ⚠️ 错误码约定

| Code | 场景示例 |
| ------ | --------- |
| `INVALID_ARGUMENT` | 缺少必填字段、ID 非数字、分页参数超限 |
| `NOT_FOUND` | 用户/儿童/监护关系不存在 |
| `ALREADY_EXISTS` | 重复创建监护关系或外部账号映射 |
| `FAILED_PRECONDITION` | 当前状态不允许操作（如封禁用户后仍更新资料） |
| `PERMISSION_DENIED` | 调用方 token 无权访问该 service |
| `UNAUTHENTICATED` | 没有或非法服务凭证 |
| `INTERNAL` | 依赖（数据库/缓存等）异常 |

错误 message 会附带可读描述，必要时在 metadata 中返回细分 `error-code`。

---

## 🧑‍💻 Go 调用示例

```go
package main

import (
    "context"
    "crypto/tls"
    "log"
    "os"
    "time"

    "github.com/google/uuid"
    identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/metadata"
)

func main() {
    conn, err := grpc.Dial(
        "iam-apiserver.internal:9090",
        grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: false})),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
    defer cancel()
    ctx = metadata.AppendToOutgoingContext(ctx,
        "authorization", "Bearer "+os.Getenv("IAM_SERVICE_TOKEN"),
        "x-request-id", "trace-"+uuid.NewString(),
    )

    readClient := identityv1.NewIdentityReadClient(conn)
    userResp, err := readClient.GetUser(ctx, &identityv1.GetUserRequest{UserId: "1024"})
    if err != nil {
        log.Fatalf("GetUser failed: %v", err)
    }
    log.Printf("User %s status=%s", userResp.User.Id, userResp.User.Status)

    guardClient := identityv1.NewGuardianshipQueryClient(conn)
    listResp, err := guardClient.ListChildren(ctx, &identityv1.ListChildrenRequest{
        UserId: "1024",
        Page:   &identityv1.OffsetPagination{Limit: 20, Offset: 0},
    })
    if err != nil {
        log.Fatalf("ListChildren failed: %v", err)
    }
    for _, edge := range listResp.Items {
        log.Printf("Child %s relation=%s", edge.Guardianship.ChildId, edge.Guardianship.Relation)
    }
}
```

---

## 🧪 调试与代码生成

- 运行 `make proto-gen` 生成 Go SDK；若需 TS/Python，可在 `Makefile` 中新增命令。
- grpcurl 示例：

```bash
grpcurl \
  -import-path api/grpc \
  -proto iam/identity/v1/identity.proto \
  -H "authorization: Bearer ${IAM_SERVICE_TOKEN}" \
  -H "x-request-id: demo-123" \
  iam-apiserver.internal:9090 \
  iam.identity.v1.IdentityRead/SearchUsers \
  '{"keyword":"138****","page":{"limit":10,"offset":0}}'
```

---

## 🚧 下一步

- 持续用 proto 生成与契约校验约束实现面，避免再次出现“合同大于运行时”的漂移。
- 引入 proto lint（buf 等）与 ABI 兼容性检查。
- 生成多语言 SDK，方便其它系统复用。
