# GitHub Actions Workflows

最后检查日期：2026-08-07

本目录维护 IAM 仓库的 CI、生产部署、生产健康检查和数据库运维 workflow。当前生产部署目标以 serverB (`SVRB_*`) 为准；`SVRA_*` 仅作为部分 SSH secret 的迁移期 fallback。

## Workflow 概览

| Workflow | 状态 | 触发方式 | 用途 |
| --- | --- | --- | --- |
| `ci.yml` | 保留 | push `main`/`develop`、PR 到 `main`、手动 | lint、test、build、coverage、短期构建 artifact |
| `cd.yml` | 保留 | `CI` 在 `main` 成功后自动；手动 | 发布镜像、生成部署包、部署 `iam-apiserver` 到 serverB |
| `concurrency-tests.yml` | 保留 | 手动；相关 MySQL/测试路径变更时 push/PR 到 `main` | MySQL-backed 并发仓储测试 |
| `db-ops.yml` | 保留，已移除通用 `migrate` | 每天 17:00 UTC；手动数据库操作 | 数据库备份、恢复、状态检查及受控遗留表退役 |
| `server-check.yml` | 保留 | 每 30 分钟；手动 | 生产容器、内部健康检查、依赖连通性 |
| `test-ssh.yml` | 保留 | 手动 | 生产主机 SSH 和基础环境诊断 |

## CI/CD 拆分

IAM 的发布流程已按 `qs-server` 项目的方式拆分：

```text
ci.yml
  -> lint
  -> test
  -> build

cd.yml
  -> make cd-image（GHCR + Docker Hub + 阿里云 ACR）【GitHub-hosted】
  -> deploy on Mac mini（group: qlume, macOS/ARM64）
       -> ACR pull --platform linux/amd64 + save tarball
       -> 公网 SCP -> serverB docker load + compose up
```

与 qs-operating-system / qlume 共用组织级 Mac mini runner（`qlume` group）、ACR Secrets；大文件上传优先 `SVRB_PUBLIC_HOST`，公网预检失败时自动回退 `SVRB_HOST`。

脚本入口：

| 脚本 | 用途 |
| --- | --- |
| `scripts/cd/image-metadata.sh` | 统一 `SERVICE=apiserver` 的镜像名、compose service、包名和健康检查元数据 |
| `scripts/cd/build-image.sh` | 使用 buildx 构建并推送 GHCR 镜像，带 `latest` 和提交 SHA tag |
| `scripts/cd/push-dockerhub.sh` | 将 GHCR 镜像同步到 Docker Hub |
| `scripts/cd/push-acr.sh` | 将 GHCR 镜像同步到阿里云 ACR |
| `scripts/cd/export-image.sh` | Mac mini 从 ACR 有界重试 pull，失败后回退 GHCR，并导出 tarball |
| `scripts/cd/setup-runner-ssh.sh` | 隔离 SSH config（`$RUNNER_TEMP`）+ serverB 主机名校验；公网不可达时回退 Tailscale 地址 |
| `scripts/cd/runner-upload-and-deploy.sh` | SCP 部署包与镜像 tarball 到 serverB |
| `scripts/cd/prepare-package.sh` | 生成 `deploy-package-apiserver.tar.gz` 和生产 `config.prod.env` |
| `scripts/cd/remote-deploy.sh` | 在 serverB 解包、`docker load`、compose up 并健康检查 |

对应 `Makefile` 入口：

```bash
make cd-validate SERVICE=apiserver
make cd-image SERVICE=apiserver DEPLOY_SHA=<sha> DEPLOY_REF=<ref>
make cd-export-image SERVICE=apiserver DEPLOY_SHA=<sha>
make cd-package SERVICE=apiserver
make cd-remote-deploy SERVICE=apiserver IMAGE_TAG=<sha>
```

组织 Secrets / Variables（与 qs-ops / qlume 共用）：`ALIYUN_ACR_*`、`SVRB_PUBLIC_HOST`、`SVR_MINI_SSH_KEY`（可选）、`SVRB_*`。原 `QS_DEPLOY_RUNNER=serverd` 与 `IAM_SERVER_DEPLOY_KEY` 已不再需要（Mac mini 用 HTTPS checkout）。

## 当前生产架构假设

`cd.yml` 部署到 serverB，并通过 `build/docker/docker-compose.prod.yml` 启动 `iam-apiserver`。容器当前只 `expose` Docker 网络内端口：

- `9080`: HTTP REST API、`/healthz` 和内部 `/readyz`
- `9090`: gRPC 服务
- `9091`: gRPC 健康检查

因此生产 Docker liveness 不再探测宿主机 `localhost:8080` 或 `localhost:9444`，而是在 `iam-apiserver` 容器内访问 `http://127.0.0.1:9080/healthz`。部署成功还必须在同一容器内通过 `/readyz`。

## ci.yml

CI 使用 `go.mod` 里的 Go 版本，不在 workflow 里硬编码 Go 版本。

关键行为：

- `lint` 运行 `make lint`。
- `test` 运行 `go test -v -race -coverprofile=coverage.out -covermode=atomic ./...`。
- coverage 通过 `codecov/codecov-action@v6` 上传，私有仓库或严格配置建议设置 `CODECOV_TOKEN`。
- `build` 在 lint/test 成功后运行 `make build`，并上传 `bin/apiserver` 作为短期 artifact。

## cd.yml

生产部署在 `main` 分支 CI 成功后自动触发，也可手动触发。

执行顺序：

```text
Build and Push Docker Image
  -> Deploy to Production
  -> Deployment Summary
```

关键行为：

- `docker` 运行 `make cd-image`，发布 GHCR 镜像并同步 Docker Hub tag。
- `deploy` 运行 `make cd-package`，上传 `deploy-package-apiserver.tar.gz` 到 serverB。
- 远端仅通过部署包内的 `scripts/cd/remote-deploy.sh` 执行部署逻辑。
- GHCR 登录失败时，远端脚本会尝试回退到 Docker Hub 镜像。
- 部署脚本先在容器内探测 `127.0.0.1:9080/healthz`，再探测
  `127.0.0.1:9080/readyz`；两者均返回 `200` 才宣布成功。
- `/readyz` 失败只让本次发布失败并输出稳定组件状态，不触发自动 restart
  或自动 rollback。

## concurrency-tests.yml

该 workflow 只覆盖 MySQL-backed 并发测试，避免每次普通文档或无关代码变更都启动 MySQL 服务。

自动触发路径：

- `go.mod`
- `go.sum`
- `internal/apiserver/infra/mysql/**`
- `internal/apiserver/application/authn/**`
- `internal/apiserver/domain/authn/**`
- `internal/apiserver/container/authn/**`
- `internal/apiserver/infra/token/keyset/**`
- `internal/apiserver/options/**`
- `internal/apiserver/maintenance/**`
- `cmd/iam-maintenance/**`
- `internal/pkg/migration/**`
- `internal/apiserver/testhelpers/**`
- `internal/pkg/code/**`
- `scripts/oneoff/**`
- `.github/workflows/concurrency-tests.yml`

它使用 `actions/setup-go@v6` 的 `go-version-file: go.mod`，并运行：

```bash
go test ./internal/apiserver/infra/mysql/... -run "Concurrent|Concurrency" -v -count=1
go test ./internal/pkg/migration -run "TestJWKSSingleActiveMigrationMySQL" -v -count=1
go test ./internal/pkg/migration -run "TestRetireSchemaVersionMigrationMySQL|TestRetireUnusedPlatformTablesMigrationMySQL|TestRetireUnusedAuditTablesMigrationMySQL|TestRetireLegacyAuthNTablesMigrationMySQL" -v -count=1
go test ./internal/pkg/migration -run "TestFullMigrationChainAndBootstrapMySQL" -v -count=1
go test ./internal/apiserver/maintenance -run "TestAuthNLegacyReconciliationMySQL" -v -count=1
go test ./scripts/dbops -run "TestLegacyRetirementPreflightAuthNMySQL" -v -count=1
```

## db-ops.yml

该 workflow 运行在 GitHub-hosted Runner 上，因此 SSH 目标优先使用 `SVRB_PUBLIC_HOST`；只有公网入口未配置时才回退到 `SVRB_HOST` / `SVRA_HOST`。

支持操作：

- `backup`: 备份 MySQL，保留最近 3 份。
- `restore`: 从 `iam_backup_YYYYMMDD_HHMMSS.sql.gz` 恢复。
- `status`: 只输出 MySQL 客户端版本、连接状态、库总大小、表数量、备份数量和最新备份时间。
- `migration-status`: 只运行轻量状态查询，额外输出 `schema_migrations` 的 version/dirty/行数、`auth_accounts/auth_credentials_legacy` 的存在性，以及 golang-migrate advisory lock 的持有类别和持续时间；不输出 SQL 原文，也不运行 AuthN 对账或历史表退役预检，用于发布中迁移诊断。
- `performance-schema-status`: 只读输出启用状态、持久化加载、TLS/X.509 前置条件、可见权限、数据库部署类型、端点供应商分类，以及 Performance Schema、`sys.schema_table_statistics` 和阿里云 RDS `information_schema.TABLE_STATISTICS` 三条表 I/O 汇总路径的只读访问能力；不输出账号、地址、grant 原文或数据库错误详情。
- `retire-identity-dry-run`: 校验版本 18 clean、指定备份新鲜且完整、两张旧表同时存在及数据库对象零依赖，不写数据库。
- `retire-identity-apply`: 在相同门禁和生产 environment 下，要求确认令牌 `RETIRE_CHILDREN_GUARDIANSHIPS`，用最终一条 DROP 同时删除 `children/guardianships`。
- `reconcile-authn-dry-run`: 在已部署的 `iam-apiserver` 容器内运行统一维护程序，只输出 AuthN 聚合对账与缺失插入计划，不写数据库。
- `reconcile-authn-verify`: 执行相同的只读对账，但只在 `retirement_eligible=true` 时成功；用于 apply 后和稳定窗口结束时的机器门禁。`status` 的 `authn/all` scope 会先运行该 v5 verify，并通过同一台主机上的短期 `0600` 证据文件交给后续预检；预检消费后立即删除。
- `reconcile-authn-apply`: 要求两小时内的完整备份和确认令牌 `BACKFILL_AUTHN_LEGACY_MISSING`，按 `authn_batch_size`（默认 5,000，上限 50,000）只补 canonical AuthN 缺失记录；已有身份状态、密码材料和锁定事实一律不覆盖，硬冲突时本批不写入。format v5 的 apply（包括零写入重跑）始终返回 `verification_required=true` 且不自行宣称可退役，必须再运行独立只读 verify。

已废弃：

- `migrate`: 当前生产镜像没有安装 `migrate` CLI，旧 job 只是尝试在容器内找二进制，实际不可依赖。迁移应通过应用启动流程或后续专门的迁移机制处理。

备份策略：

```yaml
时间: 每天 17:00 UTC / 北京时间次日 01:00
保留: 最近 3 份
位置: /opt/backups/iam/database/
格式: iam_backup_YYYYMMDD_HHMMSS.sql.gz
```

安全约束：

- 优先使用生产宿主机预装的官方 MySQL 8.x `mysql` 与 `mysqldump`；缺失时通过 passwordless sudo Docker 使用官方 `mysql:8.0` 客户端镜像，workflow 不执行包管理器安装。
- 容器客户端仍只挂载临时 `0600` defaults file，执行结束即删除容器和临时 wrapper；镜像拉取失败或 Docker 不可用时 fail closed。
- 四个 job checkout 仓库后统一通过 `scripts/dbops/database-operation.sh` 执行，不在 workflow 内复制数据库脚本。
- 使用临时 MySQL defaults file 传递凭据，避免在命令行参数中直接出现密码。
- 备份目录权限固定为 `0700`，defaults file 和最终备份固定为 `0600`。
- dump 流式写入临时 gzip，只有非空且 `gzip -t` 成功后才原子改名；失败不留下最终文件。
- 成功落盘后才执行保留最近 3 份；日志只包含 operation、时间、大小、数量和结果。
- `restore` 只接受 `iam_backup_YYYYMMDD_HHMMSS.sql.gz` 精确格式的备份文件名。
- restore 拒绝路径分隔符、符号链接、缺失文件和损坏 gzip；不会自动触发生产恢复。
- `mysqldump`/`mysql` 的原始 stderr 不进入工作流输出，避免 SQL、地址或凭据泄露。
- Identity 直接退役是一次性例外：业务所有者已明确放弃旧表到 canonical 表的数据对账，脚本不会写 `profiles/profile_links`；它仍要求指定两小时内的完整备份、`version=18, dirty=0`、零数据库对象依赖和显式确认令牌。删除后由 `000019` 幂等登记版本 19。
- AuthN 对账和补缺共用 `iam-maintenance reconcile-authn-legacy`：canonical 表为权威，只允许普通 `INSERT` 补缺。format v5 将旧 OAuth `(provider, app_id, idp_identifier)` 迁移为带 `legacy_identifier_semantics=openid_or_unionid` 标记的 LoginIdentity；apply 只提交有界写批，最终资格必须由新快照 verify 证明。整个生命周期持有 MySQL server-side named lock。无 canonical 权威的跨 owner 重复键、无效键和未知类型 fail closed，已有 canonical owner 不被覆盖。缺少 `auth_accounts` 的凭据因旧查询本就依赖 JOIN 而标记为 runtime-unreachable，由执行前完整备份承接历史恢复需求。该准备操作不删除旧表；`000023` 的生产 DROP 仍要求 fresh verify、`authn_tables` owner waiver、零依赖、完整备份和独立发布验收。

MySQL 8 workflow 使用同一脚本执行合成 backup → drop → restore → Identity dry-run/apply → 数据断言，并独立运行 AuthN dry-run/apply/冲突回滚语义测试；不接触生产数据，也不上传备份 artifact。

历史表退役证据由 `scripts/dbops/legacy-retirement-preflight.sh`（`make db-retirement-preflight`）只读采集。手动执行 `db-ops.yml` 的 `status` 时会在普通数据库状态检查后运行 format v5 预检，并要求调用者通过 `image_sha` 记录当前生产镜像；`retirement_scope` 可选择 `identity/schema_version/platform/audit/authn/all`，发布时只运行当前批次，避免无关大表对账占用窗口。AuthN scope 的数据资格来自同一任务刚完成的 format v5 verify，避免重复运行已被 v5 取代且可能超时的旧 AuthN parity；其余 scope 仍使用预检内置 parity。定时备份、`backup` 和 `restore` 不运行该预检。脚本不执行删表或数据修复，只输出 migration、精确行数、结构指纹、列契约、表 I/O、依赖计数、旧表到 canonical 表的聚合对账和逐表 eligibility；表 I/O 优先读取 Performance Schema，再降级到 `sys.schema_table_statistics`，阿里云 RDS 若隐藏两者则按官方 Object statistics 契约读取启用了 `opt_tablestat` 的 `information_schema.TABLE_STATISTICS`。三者均不可用仍 fail closed；零 I/O 不能脱离计数器生命周期与证据窗口解释为“可删除”。

`retirement_io_waiver` 是业务所有者明确批准后的审计开关。`platform_tables` 只允许 `schema_version/tenants/data_dictionary`，`audit_tables` 只允许 `operation_logs/audit_logs/auth_token_audit`，`authn_tables` 只允许 `auth_accounts/auth_credentials_legacy` 在 Performance Schema 或表 I/O 统计不可用时继续评估。任何 waiver 都不能覆盖非零 I/O、dirty migration、版本门禁、数据对账失败或数据库对象依赖，也不能用于 Identity。默认值 `none` 保持 fail closed。

## server-check.yml

生产健康检查每 30 分钟运行一次，也可手动触发。它运行在 GitHub-hosted Runner 上，目标主机优先使用 `SVRB_PUBLIC_HOST`；只有公网入口未配置时才回退到 `SVRB_HOST` / `SVRA_HOST`。

检查项：

- 主机 CPU、内存、磁盘、负载、Top CPU 进程。
- Docker daemon 是否可用。
- `iam-apiserver` 容器是否运行。
- Docker healthcheck 是否 healthy；unhealthy 时会输出日志并尝试重启一次。
- `infra-network` 是否存在。
- 容器内 `http://127.0.0.1:9080/healthz` 是否返回 200。
- 容器内 `/readyz` 是否可接流量；失败只告警，不因 readiness 单独重启。
- 从容器内对 MySQL 和 Redis 做 TCP 连通性检查。

## test-ssh.yml

手动 SSH 诊断入口，用于验证生产主机连接、时区、Docker、`iam-apiserver` 容器和基本资源状态。它运行在 GitHub-hosted Runner 上，也优先使用 `SVRB_PUBLIC_HOST`。它不替代 `server-check.yml`，只用于排查 SSH secret、网络入口或主机基础环境。

## Variables 与 Secrets

生产 SSH 目标的主机/账号/端口使用组织 **Variables**；私钥与 sudo 密码使用 **Secrets**。

| Variable | 必需性 | 用途 |
| --- | --- | --- |
| `SVRB_PUBLIC_HOST` | 推荐 | serverB 公网 SSH 地址；GitHub-hosted 健康检查、数据库运维、SSH 诊断和部署均优先使用 |
| `SVRB_HOST` | 回退 | serverB Tailscale 地址；Mac mini 部署在公网预检失败时自动使用，GitHub-hosted Runner 通常无法直接访问 |
| `SVRB_USERNAME` | 推荐 | serverB SSH 用户；缺省用 `SVRA_USERNAME` |
| `SVRB_SSH_PORT` | 可选 | serverB SSH 端口；缺省用 `SVRA_SSH_PORT` 或 22 |
| `SVRA_HOST` | 可选 | fallback 主机地址 |
| `SVRA_USERNAME` | 推荐 | fallback SSH 用户 |
| `SVRA_SSH_PORT` | 可选 | fallback SSH 端口 |

| Secret | 必需性 | 用途 |
| --- | --- | --- |
| `SVR_MINI_SSH_KEY` | 推荐 | Mac mini 部署优先使用的 SSH 私钥 |
| `SVRB_SSH_KEY` | 回退 | serverB SSH 私钥；缺省用 `SVRA_SSH_KEY` |
| `SVRB_SUDO_PASSWORD` | 可选 | serverB sudo 密码；缺省用 `SVRA_SUDO_PASSWORD` |
| `SVRA_SSH_KEY` | 推荐 | fallback SSH 私钥 |
| `MYSQL_HOST` | 必需 | MySQL 主机 |
| `MYSQL_PORT` | 可选 | MySQL 端口，默认 3306 |
| `MYSQL_USERNAME` | 必需 | MySQL 用户 |
| `MYSQL_PASSWORD` | 必需 | MySQL 密码 |
| `MYSQL_DBNAME` | 必需 | MySQL 数据库 |
| `REDIS_HOST` | 必需 | Redis 主机 |
| `REDIS_PORT` | 可选 | Redis 端口，默认 6379 |
| `REDIS_DB` | 可选 | Redis DB，默认 0 |
| `REDIS_USERNAME` | 可选 | Redis 用户 |
| `REDIS_PASSWORD` | 可选 | Redis 密码 |
| `REDIS_USE_SSL` | 可选 | Redis TLS 开关 |
| `REDIS_SSL_INSECURE_SKIP_VERIFY` | 可选 | Redis TLS 跳过校验 |
| `IPD_ENCRYPTION_KEY` | 必需 | IDP 加密密钥 |
| `DOCKERHUB_USERNAME` | 必需 | Docker Hub 同步和 GHCR fallback |
| `DOCKERHUB_TOKEN` | 必需 | Docker Hub 同步和 GHCR fallback |
| `WWW_UID` | 必需 | 容器运行 UID |
| `WWW_GID` | 必需 | 容器运行 GID |
| `CODECOV_TOKEN` | 可选 | Codecov 上传 token |
| `SMS_ALIYUN_ACCESS_KEY_ID` | 可选 | 阿里云短信 AK；仅当短信 provider 配置为 `aliyun` 且不走 RAM 角色时需要 |
| `SMS_ALIYUN_ACCESS_KEY_SECRET` | 可选 | 阿里云短信 SK；必须与 `SMS_ALIYUN_ACCESS_KEY_ID` 同时配置或同时留空 |

NSQ 事件发布相关 secret 仍为可选：

- `NSQ_LOOKUPD_HOST`
- `NSQ_LOOKUPD_PORT`
- `NSQ_NSQD_HOST`
- `NSQ_NSQD_PORT`

## 常用操作

手动运行 CI：

```text
Actions -> CI -> Run workflow
```

手动生产部署：

```text
Actions -> Production Deploy -> Run workflow
```

手动健康检查：

```text
Actions -> Production Health Check -> Run workflow
```

数据库备份：

```text
Actions -> Database Operations -> Run workflow -> backup
```

数据库恢复：

```text
Actions -> Database Operations -> Run workflow -> restore
backup_name = iam_backup_YYYYMMDD_HHMMSS.sql.gz
```

SSH 诊断：

```text
Actions -> Production SSH Diagnostics -> Run workflow
```

## 维护原则

- 新 workflow 应优先读取 `go.mod`，不要硬编码 Go 版本。
- GitHub-hosted workflow 的生产 SSH 目标必须优先使用 `SVRB_PUBLIC_HOST`；`SVRB_HOST` 是 Tailscale 回退地址。账号/端口使用组织 Variables，私钥与 sudo 密码仍用 Secrets。
- GitHub secrets 只在 workflow 中解引用；脚本通过普通环境变量接收。
- CI/CD 运行步骤应优先放入 `scripts/cd` 和 `Makefile cd-*`，避免在 YAML 里堆叠长脚本。
- 健康检查应贴合 compose 真实网络模型：容器内探测 `9080`，不要恢复宿主机 `8080/9444` 探测。
- 临时诊断 workflow 不应加 schedule；定时检查统一放在 `server-check.yml`。
- 如果引入新的迁移机制，应新增明确 workflow，而不是恢复旧的 `db-ops migrate` 分支。
