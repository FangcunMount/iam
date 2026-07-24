# Keys (JWKS) 目录说明

本目录用于存放 IAM 服务在开发环境中使用的本地私钥（PEM 格式）。生产环境建议使用 KMS/HSM 或其他受管密钥存储方案，本地文件仅用于开发、测试或离线排查。

## 主要要点

- 默认开发位置（示例）：`configs/keys`
- 运行时由配置项 `jwks.keys_dir` 指定（`apiserver.prod.yaml`/`apiserver.dev.yaml`）。
- 服务启动时可通过配置 `jwks.auto_init: true` 自动创建第一个活跃密钥（仅在开发场景推荐）。
- 私钥文件格式：PKCS#8 PEM（未加密或加密均可，但代码期望可直接读取并解析为 RSA 私钥）。
- 支持的文件名：`{kid}.pem` 或 `key-{kid}.pem`（解析器会尝试这两种命名以兼容历史）。
- 文件权限：建议 `0600`，仅允许服务运行用户读取。

## 配置示例

在 `configs/apiserver.dev.yaml` 中确保包含：

```yaml
jwks:
  keys_dir: ./configs/keys
  auto_init: true
```

启动服务时，若目录为空且 `auto_init` 为 `true`，服务会尝试创建一把活跃密钥并写入该目录。

## 手动生成私钥（快速方法）

如果需要手工在 `keys_dir` 中放置私钥，可以使用 OpenSSL：

1. 生成 RSA 私钥（2048 位示例）：

```bash
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out key-<kid>.pem
chmod 600 key-<kid>.pem
```

2. （可选）导出公钥（用于调试）：

```bash
openssl rsa -in key-<kid>.pem -pubout -out pub-<kid>.pem
```

注意：如果你先生成一个传统 RSA 私钥（例如 `rsa_priv.pem`），可以用下面的命令将其转换为 PKCS#8：

```bash
openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt -in rsa_priv.pem -out key-<kid>.pem
chmod 600 key-<kid>.pem
```

将 `<kid>` 替换为你想要的密钥 ID（建议使用 UUID 或短哈希）。如果系统中已有 JWKS 数据库存储（例如数据库记录中有 kid），请使用与数据库记录一致的 kid 命名私钥文件以便解析器能够正确找到私钥。

## 通过服务 API 创建密钥（推荐）

在多数场景下，建议使用系统提供的“创建密钥”接口或服务启动的自动初始化流程：

- 启动服务并设置 `jwks.auto_init: true`，服务会在 `keys_dir` 下写入自动生成的私钥文件并在数据库中登记对应的 JWK/Key 记录。
- 如果需要主动通过 REST API 创建密钥，请使用 JWKS 管理接口（`internal/apiserver/transport/rest/authn/handler/jwks_admin_keys.go`）。创建操作会原子激活新密钥、把旧 active 转为 grace，并在数据库和 `keys_dir` 中保存对应事实。

## 文件名约定与解析规则

解析器会尝试下面两种私钥文件名（以 `kid` 为例）：

- `${keys_dir}/${kid}.pem`
- `${keys_dir}/key-${kid}.pem`

因此：

- 如果你手动放置私钥，优先使用 `key-<kid>.pem` 格式以保证兼容性。
- 私钥需要能被服务运行用户读取；若权限不当会导致 "private key file not found: ..." 或读取失败错误。

## 轮换与退役（建议流程）

1. 通过管理 API 创建新密钥（或等待自动轮换任务），将其设为 `active`。
2. 在 `grace`（宽限）期内继续接受用旧密钥签发的令牌，并同时在 JWKS 发布中包含旧公钥。
3. 宽限期结束后将旧密钥设为 `retired` 并从 `keys_dir` 中安全删除对应私钥（对删除操作请谨慎，确保系统不再需要该私钥）。

注意：当前多实例只支持所有签名实例共享同一个可靠 POSIX 密钥目录；多主机非共享目录部署不受支持。KMS/HSM 是后续演进边界。

## 故障排查

- 错误：`private key file not found: key-<kid>.pem`
  - 检查 `jwks.keys_dir` 是否指向正确路径（可使用绝对路径减少歧义）。
  - 检查文件名是否为 `{kid}.pem` 或 `key-{kid}.pem`，并与数据库中的 kid 对应。
  - 检查文件权限和所有者（`ls -l`，应为 `-rw-------`，服务运行用户可读）。

- 错误：JWT 验证失败 / 签名无效
  - 确认 JWKS（公钥）在服务或外部发布点（例如 `/jwks`）中包含对应 kid 的公钥。
  - 确认服务使用的私钥与公钥配对（私钥正确写入且未损坏）。
  - 检查系统时钟是否同步（NTP），时钟漂移会导致 token 被判定为未生效或已过期。

## 安全建议（生产）

- 不要在生产环境使用本地 PEM 文件存放长期密钥。使用云 KMS（AWS KMS、GCP KMS、Azure Key Vault）或企业 HSM。
- 如果必须使用文件系统，请将目录挂载到加密磁盘、限制访问到运行用户，并在运维流程中明确密钥轮换与删除策略。
- 审计密钥访问日志并限制能调用创建/退役密钥管理接口的账户。

## 常用命令小结

```bash
# 列出 keys 目录
ls -la ${JWKS_KEYS_DIR:-configs/keys}

# 手工生成 2048 位 RSA 私钥并设置权限
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out key-<kid>.pem
chmod 600 key-<kid>.pem

# 转换为 PKCS#8（如果你有老格式私钥）
openssl pkcs8 -topk8 -inform PEM -outform PEM -nocrypt -in rsa_priv.pem -out key-<kid>.pem

# 启动服务（示例）
go run ./cmd/apiserver -c ./configs/apiserver.dev.yaml
```
