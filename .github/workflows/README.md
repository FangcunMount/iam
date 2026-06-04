# GitHub Actions Workflows

最后检查日期：2026-05-03

本目录维护 IAM 仓库的 CI、生产部署、生产健康检查和数据库运维 workflow。当前生产部署目标以 serverB (`SVRB_*`) 为准；`SVRA_*` 仅作为部分 SSH secret 的迁移期 fallback。

## Workflow 概览

| Workflow | 状态 | 触发方式 | 用途 |
| --- | --- | --- | --- |
| `ci.yml` | 保留 | push `main`/`develop`、PR 到 `main`、手动 | lint、test、build、coverage、短期构建 artifact |
| `cd.yml` | 保留 | `CI` 在 `main` 成功后自动；手动 | 发布镜像、生成部署包、部署 `iam-apiserver` 到 serverB |
| `cicd.yml` | 已废弃 | 手动，仅输出替代说明 | 旧合并式 CI/CD；已由 `ci.yml` + `cd.yml` 替代 |
| `concurrency-tests.yml` | 保留 | 手动；相关 MySQL/测试路径变更时 push/PR 到 `main` | MySQL-backed 并发仓储测试 |
| `db-ops.yml` | 保留，已移除 `migrate` | 每天 17:00 UTC；手动 `backup`/`restore`/`status` | 数据库备份、恢复、状态检查 |
| `server-check.yml` | 保留 | 每 30 分钟；手动 | 生产容器、内部健康检查、依赖连通性 |
| `test-ssh.yml` | 保留 | 手动 | 生产主机 SSH 和基础环境诊断 |
| `ping-runner.yml` | 已废弃 | 手动，仅输出替代说明 | 旧快速 ping；由 `server-check.yml` 替代 |

## CI/CD 拆分

IAM 的发布流程已按 `qs-server` 项目的方式拆分：

```text
ci.yml
  -> lint
  -> test
  -> build

cd.yml
  -> make cd-image
  -> make cd-package
  -> remote scripts/cd/remote-deploy.sh
```

GitHub Actions 只负责传递 GitHub secrets、checkout、镜像仓库登录、SSH/SCP 编排；镜像构建、Docker Hub 同步、部署包生成、远端部署步骤都落在仓库脚本和 `Makefile` 目标中。

脚本入口：

| 脚本 | 用途 |
| --- | --- |
| `scripts/cd/image-metadata.sh` | 统一 `SERVICE=apiserver` 的镜像名、compose service、包名和健康检查元数据 |
| `scripts/cd/build-image.sh` | 使用 buildx 构建并推送 GHCR 镜像，带 `latest` 和提交 SHA tag |
| `scripts/cd/push-dockerhub.sh` | 将 GHCR 镜像同步到 Docker Hub |
| `scripts/cd/prepare-package.sh` | 生成 `deploy-package-apiserver.tar.gz` 和生产 `config.prod.env` |
| `scripts/cd/remote-deploy.sh` | 在 serverB 解包、同步配置、登录镜像仓库、`docker compose up` 并健康检查 |

对应 `Makefile` 入口：

```bash
make cd-validate SERVICE=apiserver
make cd-image SERVICE=apiserver DEPLOY_SHA=<sha> DEPLOY_REF=<ref>
make cd-package SERVICE=apiserver
make cd-remote-deploy SERVICE=apiserver IMAGE_TAG=<sha>
```

## 当前生产架构假设

`cd.yml` 部署到 serverB，并通过 `build/docker/docker-compose.prod.yml` 启动 `iam-apiserver`。容器当前只 `expose` Docker 网络内端口：

- `9080`: HTTP REST API 和 `/healthz`
- `9090`: gRPC 服务
- `9091`: gRPC 健康检查

因此生产健康检查不再探测宿主机 `localhost:8080` 或 `localhost:9444`，而是在 `iam-apiserver` 容器内访问 `http://127.0.0.1:9080/healthz`。

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
- 部署后的健康检查在容器内探测 `127.0.0.1:9080/healthz`。

## concurrency-tests.yml

该 workflow 只覆盖 MySQL-backed 并发测试，避免每次普通文档或无关代码变更都启动 MySQL 服务。

自动触发路径：

- `go.mod`
- `go.sum`
- `internal/apiserver/infra/mysql/**`
- `internal/apiserver/application/uc/**`
- `internal/apiserver/testhelpers/**`
- `internal/pkg/code/**`
- `.github/workflows/concurrency-tests.yml`

它使用 `actions/setup-go@v6` 的 `go-version-file: go.mod`，并运行：

```bash
go test ./internal/apiserver/infra/mysql/... -run "Concurrent|Concurrency" -v -count=1
```

## db-ops.yml

支持操作：

- `backup`: 备份 MySQL，保留最近 3 份。
- `restore`: 从 `iam_backup_YYYYMMDD_HHMMSS.sql.gz` 恢复。
- `status`: 查看数据库版本、库大小、表列表、最大表和备份列表。

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

- 使用临时 MySQL defaults file 传递凭据，避免在命令行参数中直接出现密码。
- `restore` 只接受 `iam_backup_*.sql.gz` 格式的备份文件名。
- `mysqldump` stderr 不再写入 `.sql` 文件，避免错误输出污染备份内容。

## server-check.yml

生产健康检查每 30 分钟运行一次，也可手动触发。目标主机使用 `SVRB_HOST` 优先，缺失时 fallback 到 `SVRA_HOST`。

检查项：

- 主机 CPU、内存、磁盘、负载、Top CPU 进程。
- Docker daemon 是否可用。
- `iam-apiserver` 容器是否运行。
- Docker healthcheck 是否 healthy；unhealthy 时会输出日志并尝试重启一次。
- `infra-network` 是否存在。
- 容器内 `http://127.0.0.1:9080/healthz` 是否返回 200。
- 从容器内对 MySQL 和 Redis 做 TCP 连通性检查。

## test-ssh.yml

手动 SSH 诊断入口，用于验证生产主机连接、时区、Docker、`iam-apiserver` 容器和基本资源状态。它不替代 `server-check.yml`，只用于排查 SSH secret 或主机基础环境。

## ping-runner.yml

已废弃。旧 workflow 曾每 6 小时运行一次快速 ping，但它与 `server-check.yml` 重复，并且旧逻辑依赖宿主机端口探测。当前文件保留为手动提示，避免继续定时误报。

## Secrets

生产部署和运维推荐配置：

| Secret | 必需性 | 用途 |
| --- | --- | --- |
| `SVRB_HOST` | 部署必需 | serverB 主机地址 |
| `SVRB_USERNAME` | 可选 | serverB SSH 用户；缺省用 `SVRA_USERNAME` |
| `SVRB_SSH_KEY` | 可选 | serverB SSH 私钥；缺省用 `SVRA_SSH_KEY` |
| `SVRB_SSH_PORT` | 可选 | serverB SSH 端口；缺省用 `SVRA_SSH_PORT` 或 22 |
| `SVRB_SUDO_PASSWORD` | 可选 | serverB sudo 密码；缺省用 `SVRA_SUDO_PASSWORD` |
| `SVRA_USERNAME` | 必需 | fallback SSH 用户 |
| `SVRA_SSH_KEY` | 必需 | fallback SSH 私钥 |
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
- 生产 SSH 目标使用 `SVRB_*` 优先，`SVRA_*` 只作为迁移期 fallback。
- GitHub secrets 只在 workflow 中解引用；脚本通过普通环境变量接收。
- CI/CD 运行步骤应优先放入 `scripts/cd` 和 `Makefile cd-*`，避免在 YAML 里堆叠长脚本。
- 健康检查应贴合 compose 真实网络模型：容器内探测 `9080`，不要恢复宿主机 `8080/9444` 探测。
- 临时诊断 workflow 不应加 schedule；定时检查统一放在 `server-check.yml`。
- 如果引入新的迁移机制，应新增明确 workflow，而不是恢复旧的 `db-ops migrate` 分支。
