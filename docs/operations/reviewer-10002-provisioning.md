# 固定审核账号 10002

> 状态：已实现 · 脚本与维护入口已完成；生产开通状态以每次 Actions 执行与复核回执为准，账号登录及人工审核另行验收。

使用 [专用脚本](../../scripts/ops/provision-reviewer-10002.py) 创建独立 IAM 用户 `10002`，按执行前核对的 `10001` 直接管理员角色赋权，再通过 QS 正式接口登记运营身份。默认登录名为 `review@mfangcunmount.com`，姓名为“安全与产品审核员”。用户 ID 与登录名是两个字段，不需要用 `10002` 登录。

本工具按“与 10001 同级”的要求授予完整管理员角色，权限范围大于单独的 AI 审核。当前允许核对的角色为 `platform_admin`、`iam_admin`、`qs:admin`，其中必须有前者和后者；使用实际角色 ID，不硬编码数字角色 ID。出现其他角色或重复、带范围的赋权时停止，不把业务范围转换成无限范围。共享角色后续的权限调整也会影响新账号。

已有 `admin@fangcunmount.com` 是另一登录身份；本流程不复用、不重置它。脚本不复制 10001 的密码、会话或业务数据，不提交人工审核意见。

## 推荐：通过 GitHub Actions 执行

在 IAM 仓库使用 [Provision Reviewer 10002](../../.github/workflows/reviewer-10002.yml)。Workflow 合并到 `main` 后，在 Actions 中选择它并点击 Run workflow；只允许从 `main` 执行，不由提交或定时任务自动创建账号。

在 IAM 的仓库 Secrets 或 `production` 环境 Secrets 中设置 `IAM_REVIEWER_10002_PASSWORD`（新密码，20–128 字节）及 `IAM_REVIEWER_PROVISION_TOKEN`（10001 当前有效的 access token）。复用现有 `MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USERNAME`、`MYSQL_PASSWORD`、`MYSQL_DATABASE` 或 `MYSQL_DBNAME`，以及 IAM 已有的 SVRB/SVRA SSH 维护连接。Secrets 不填入 Run workflow 的公开输入字段。

1. 首次选择 `preflight`，其他输入留空。它在服务器准备私有输入并执行只读检查，不创建账号。成功日志包含 `plan_id`、源代码 SHA 和计划 `fingerprint`。
2. 核对账号 `10002`、登录名 `review@mfangcunmount.com`、组织与管理员角色，选择 `apply`，填入相同的 `plan_id` 和 `approve_plan` 指纹。完成后应显示 `provisioned`。
3. 选择 `verify`，填入同一 `plan_id`，只读核对最终状态。投影尚未收敛时，稍后重新复核即可。

执行主机与当前 IAM 维护工作流相同，使用 SSH 用户主目录下的 `.iam-reviewer-10002`（0700）持久保存账号输入、计划和回执；新密码文件为 0600，不能上传为 Actions artifact。每次传输的临时令牌与数据库凭据在执行后清理，GitHub 日志仅显示非秘密结果。若运行被强制中断或 SSH 失联导致清理步骤失败，应通过同一维护连接移除该运行的 `iam-reviewer-upload-<run_id>-<attempt>` 临时目录；保留持久状态目录用于恢复。

同一维护并发组串行执行，服务器目录锁也覆盖手工执行。流程中断后保留同一计划重试；不能换密码或改掉现有账号输入。若预演后 `main` 的提交改变，先重新预演，不能使用旧代码版本的计划执行新代码。SSH 用户或主机变化也须先找回原受限状态，不应重新初始化。

整个 Actions 流程无需人工运行 Python，也不替代审核者本人登录和审核。下面的命令行步骤保留供维护和排障使用。

## 准备工具和执行环境

在本版本 IAM 代码目录构建维护程序。它与脚本在可访问 IAM MySQL、IAM HTTPS 和 QS HTTPS 的可信维护主机执行；不需要为此重启 IAM 服务。必须使用包含保留 ID 支持的新维护程序，不能复用旧二进制。

```bash
go build -o /tmp/iam-maintenance-reviewer ./cmd/iam-maintenance
python3 scripts/ops/provision-reviewer-10002.py --help
```

若在 macOS 构建后上传到 Linux amd64 主机，构建时指定 `GOOS=linux GOARCH=amd64`；脚本仅依赖 Python 3 标准库。将二进制与脚本保存在可信、不可被其他用户修改的目录。

通过已有安全配置方式向维护程序提供以下环境变量，不在终端或 CI 输出其内容：

| 环境变量 | 含义 |
| --- | --- |
| `IAM_APISERVER_MYSQL_HOST` | IAM 主数据库地址 |
| `IAM_APISERVER_MYSQL_PORT` | 端口，默认 3306 |
| `IAM_APISERVER_MYSQL_USERNAME` | 维护数据库账号 |
| `IAM_APISERVER_MYSQL_PASSWORD` | 数据库密码 |
| `IAM_APISERVER_MYSQL_DATABASE` | IAM 数据库名 |

这必须是 IAM 数据库，不是 QS 或 qs-ai 数据库。固定 ID 由维护专用 Signup 事务创建；公开注册接口仍由系统分配 ID。预演与执行均检查 10001 当前是有效的平台根管理员。

准备 **10001 本人的有效管理员 access token**，存入可信主机的私有普通文件，例如 `/root/reviewer-10002.token`，权限 `0600`。使用已有安全凭据流程传入，不放入命令参数、聊天、Git 或 CI 日志。赋权仍通过正式接口鉴权，审计操作人由该 token 确定；数据库维护入口的操作人固定为 10001，两者应一致。令牌过期后可替换 token 文件再继续。

默认 API 地址为 `https://iam.fangcunmount.cn/api/v4` 和 `https://qs.fangcunmount.cn/api/v1`。如环境不同，四个步骤中一致使用 `--iam-url`、`--qs-url`。只接受 HTTPS，验证证书并拒绝跳转；不关闭 TLS 验证。

## 初始化和预演

以下示例在可信 Linux 主机执行。工作目录必须是尚不存在的新目录，位于 Git 外；脚本创建权限为 `0700` 的目录。交互输入新账号独立密码并确认，20–128 字节。密码保存在 `0600` 的 `account.json`，不会显示在计划或日志中。

```bash
python3 scripts/ops/provision-reviewer-10002.py init \
  --directory /root/reviewer-10002 \
  --username review@mfangcunmount.com

python3 scripts/ops/provision-reviewer-10002.py preflight \
  --directory /root/reviewer-10002 \
  --maintenance /tmp/iam-maintenance-reviewer \
  --token-file /root/reviewer-10002.token
```

这两个步骤没有生产写入。核对输出的登录名、10002、组织和角色列表，以及私有 `plan.json` 中的角色定义与授权内容。计划绑定账号输入指纹、10001 赋权、角色授权、组织及 API 地址，角色漂移时原计划失效。已有用户名、用户 ID（包括已删除记录）或孤立赋权、运营身份不会被接管。

## 执行和复核

将下面的 `REVIEWED_PLAN_SHA256` 替换为预演输出的真实指纹：

```bash
python3 scripts/ops/provision-reviewer-10002.py apply \
  --directory /root/reviewer-10002 \
  --maintenance /tmp/iam-maintenance-reviewer \
  --token-file /root/reviewer-10002.token \
  --approve-plan REVIEWED_PLAN_SHA256

python3 scripts/ops/provision-reviewer-10002.py verify \
  --directory /root/reviewer-10002 \
  --maintenance /tmp/iam-maintenance-reviewer \
  --token-file /root/reviewer-10002.token
```

`apply` 按顺序创建 IAM User、用户名身份及密码 Credential，授予已核对的管理员角色，登记同组织 QS Operator。前三项身份事实使用正常 Signup 事务和密码算法；赋权使用 IAM `/authz/assignments/grant`，登记使用 QS `/operators`。不会直写授权表或 QS 业务库，也不恢复旧账号创建 RPC。

`provisioned` 表示账号 ID 正确、直接角色匹配、运营身份存在且管理员授权投影已收敛。`awaiting_projection` 表示尚未完成，稍后使用只读 `verify`；不能以账号已插入替代权限收敛。

开通后由实际的第二位审核者使用独立登录名和密码登录 Operating，确认进入 AI 治理与“安全与产品”批量审核。登录验收与实际审核记录另行确认，脚本的回执明确标记 `login_and_human_review: not_performed`。不要由脚本代签或借用 10001 的会话提交第二域审核。

## 中断恢复与记录

- 保留同一份 `account.json`、计划和受限回执，始终使用同一工作目录。目录锁阻止同目录并发执行；不要复制到其他目录同时运行。
- HTTP 超时不会立即重发写入。先执行 `verify` 查看情况；确认计划未变化后，可重新执行相同 `apply`。已有本次账号、匹配角色和运营身份会跳过，不重复创建、不重置密码。
- 跨 IAM 与 QS 无分布式事务，中断可能留下已创建身份或部分角色。每次写入前保存 intent，结果不明时停止；不自动撤权、删除用户或覆盖状态。
- 10001 的角色定义、赋权、组织或 API 地址变更时停止。核对变更后使用 `preflight --plan /root/reviewer-10002/plan-next.json` 生成新计划，后续 `apply`、`verify` 使用同一 `--plan`。原计划和回执不覆盖。
- 目标被别人修改、密码已轮换、账号状态改变或存在不在计划内的角色时，停止并人工核对。不能通过重新初始化、换请求 ID 或改库来绕过占用检查。
- `account.json` 含密码，只在开通与恢复期间受限保存。验收完成后通过正式安全渠道交付账号，妥善处理明文凭据与过期 token；非秘密计划和回执可单独受限归档。密码改变后原输入不再满足历史完成校验。

## 验证范围

Python 测试覆盖预演无写入、指纹拒绝、角色及组织漂移、响应丢失后的恢复、重复执行、投影延迟、分页完整性和私有文件边界。维护模块的真实 MySQL 测试覆盖固定 ID、占用拒绝、事务回滚、重复执行不改密及权限撤销。CI 分别运行脚本单元测试和既有 MySQL 维护测试。

这些测试不代表生产账号已开通，也不代表新账号已经登录或完成 AI 审核；生产证据以实际执行回执、权限收敛与本人操作为准。
