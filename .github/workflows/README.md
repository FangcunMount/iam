# GitHub Actions Workflows

本项目使用 GitHub Actions 实现自动化 CI/CD 流程和运维监控，采用 Docker 容器化部署架构。

## 📋 目录

- [工作流概览](#工作流概览)
- [环境配置](#环境配置)
- [Secrets 配置](#secrets-配置)
- [使用指南](#使用指南)
- [故障排查](#故障排查)
- [最佳实践](#最佳实践)

---

## 工作流概览

### 1. **cicd.yml** - 主 CI/CD 流程

- **触发方式**:
  - Push 到 main/develop 分支
  - Pull Request 到 main 分支
  - 手动触发 (workflow_dispatch)
- **运行时间**: ~10-15 分钟
- **执行流程**:

```text
Validate Secrets (验证配置)
  ↓
Test (单元测试) ━━━┓
                   ┣━━→ Parallel
Lint (代码检查) ━━━┛
  ↓
Build (编译构建)
  ↓
Docker (镜像构建) ← 仅 main 分支，推送到 ghcr.io
  ↓
Deploy (部署到生产) ← 仅 main 分支
  ↓
Health Check (健康验证，最长 150 秒)
```

**部署步骤详解**:

1. SSH 连接到生产服务器 (SVRA)
2. 备份当前版本到 `/opt/backups/iam/deployments/`
3. 拉取最新 Docker 镜像 `ghcr.io/fangcunmount/iam:latest`
4. 停止现有容器 (iam-apiserver)
5. 清理旧容器和镜像
6. 启动新容器（端口映射 8080:9080, 9444:9444）
7. 健康检查（轮询 `/healthz` 端点）
8. 验证部署成功

**关键配置**:

- Go 版本: 1.24
- Docker Registry: ghcr.io
- Image: `fangcunmount/iam:latest`
- 健康检查超时: 150 秒
- 容器资源配额：直接维护在 `build/docker/docker-compose.prod.yml`
- 应用连接池与后台任务频率：直接维护在 `configs/apiserver.prod.yaml`

---

### 2. **db-ops.yml** - 数据库操作

- **触发方式**:
  - **自动触发**: 每天北京时间凌晨 01:00（UTC 17:00）自动备份
  - **手动触发**: workflow_dispatch，支持 4 种操作
- **运行时间**: 1-5 分钟（视操作而定）
- **支持操作**:
  - `backup`: 备份数据库（保留最近 **3 次**备份）
  - `restore`: 从指定备份恢复数据库
  - `migrate`: 运行数据库迁移（在 Docker 容器内执行）
  - `status`: 查看数据库状态和可用备份

**自动备份策略**:

```yaml
时间: 每天北京时间 01:00
保留: 最近 3 次备份
位置: /opt/backups/iam/database/
格式: iam_backup_YYYYMMDD_HHMMSS.sql.gz
```

**使用方法**:

```bash
# 手动触发
Actions → Database Operations → Run workflow

# 选择操作:
- backup: 立即备份（无需参数）
- restore: 恢复备份（需输入文件名，如 iam_backup_20231024_010000.sql.gz）
- migrate: 数据库迁移
- status: 查看状态和备份列表
```

**安全特性**:

- ✅ 使用环境变量传递密码，避免暴露在日志中
- ✅ 备份包含存储过程、触发器（--routines, --triggers）
- ✅ 使用事务一致性备份（--single-transaction）
- ⚠️ 恢复操作有 5 秒延迟和警告提示

---

### 3. **server-check.yml** - 服务器健康检查

- **触发方式**:
  - 自动触发: 每 30 分钟执行一次
  - 手动触发: workflow_dispatch
- **运行时间**: ~2-3 分钟
- **检查内容**:

**系统健康**:

- CPU 使用率
- 内存使用情况（已用/总量/百分比）
- 磁盘使用（根分区，>80% 触发警告）
- 系统负载（Load Average）
- Top 5 CPU 占用进程

**Docker 服务**:

- Docker daemon 状态
- IAM 容器运行状态
- 容器健康检查状态（healthy/unhealthy）
- **自动恢复**: 检测到 unhealthy 容器自动重启

**网络与 API**:

- 端口监听状态（8080, 9444）
- HTTP API 健康检查（localhost:8080/healthz）
- HTTPS API 健康检查（localhost:9444/healthz）

**数据库与 Redis**:

- MySQL 连接测试（使用环境变量，安全）
- Redis 连接测试
- 不暴露密码到日志

**告警机制**:

- 磁盘使用 >80%: 触发警告
- Docker 未运行: 任务失败
- 容器 unhealthy: 自动重启并记录
- API 无响应: 任务失败

---

### 4. **ping-runner.yml** - 快速连通性检查

- **触发方式**:
  - 自动触发: 每 6 小时执行一次
  - 手动触发: workflow_dispatch
- **运行时间**: ~1 分钟
- **检查内容**:

**生产服务器 (SVRA)**:

- 系统状态（主机名、运行时间、日期）
- 资源概览（内存、磁盘、CPU、负载）

**Docker 服务**:

- Docker daemon 状态
- 所有运行中的容器列表
- IAM 容器详细状态

**API 健康**:

- HTTP API (8080): 健康检查
- HTTPS API (9444): 健康检查

**GitHub Runner**:

- Runner 信息（OS、名称、架构）

**特点**:

- 轻量级快速检查
- 并行执行（生产服务器 + GitHub Runner）
- 提供整体状态报告

---

### 5. **test-ssh.yml** - SSH 连接测试（新增）

- **触发方式**: 仅手动触发 (workflow_dispatch)
- **运行时间**: ~1 分钟
- **用途**: 验证 SSH 配置和服务器状态

**检查内容**:

**GitHub Runner 信息**:

- Runner OS、架构、名称
- **UTC 时间**（用于验证 cron 时间计算）

**SSH 连接测试**:

- 服务器基本信息（主机名、用户、工作目录）

**时区信息**（重要）:

- 服务器本地时间
- UTC 时间
- 时区配置（Asia/Shanghai 等）
- 用于验证自动备份时间是否正确

**系统信息**:

- 操作系统和内核版本
- 系统运行时间

**Docker 状态**:

- Docker 版本
- 运行中的容器数量

**IAM 服务状态**:

- 容器运行状态
- 容器详细信息

**资源使用**:

- 磁盘空间（根分区）
- 内存使用情况

**使用场景**:

- ✅ 验证 SVRA_* Secrets 配置是否正确
- ✅ 确认服务器时区（验证自动备份时间）
- ✅ 排查 SSH 连接问题
- ✅ 快速诊断服务器和服务状态
- ✅ 验证 cron 时间计算（UTC vs 北京时间）

---

## 工作流时间表

| 工作流 | 触发方式 | 频率 | 用途 |
| -------- | --------- | ------ | ------ |
| **cicd.yml** | push/PR/手动 | 按需 | 持续集成和部署 |
| **db-ops.yml** | **自动**/手动 | **每天 01:00** | 数据库备份和操作 |
| **server-check.yml** | 自动/手动 | 每 30 分钟 | 深度健康检查 |
| **ping-runner.yml** | 自动/手动 | 每 6 小时 | 快速连通性检查 |
| **test-ssh.yml** | 仅手动 | - | SSH 和时区验证 |

**⏰ 时区说明**:

- GitHub Actions cron 使用 **UTC 时间**
- `0 17 * * *` (UTC 17:00) = **北京时间 01:00**（次日）
- 服务器备份文件时间戳使用**服务器本地时间**

---

## 环境配置

### 当前架构

```text
开发环境 (MacBook)
    ↓ git push
GitHub (CI/CD)
    ↓ Docker deploy
生产环境 (SVRA)
  ├─ Docker: iam-apiserver
  ├─ MySQL: RDS
  └─ Redis: Container
```

### 技术栈

**开发与构建**:

- **Go**: 1.24
- **框架**: Gin v1.10.1
- **构建**: Docker multi-stage build
- **镜像仓库**: GitHub Container Registry (ghcr.io)

**部署架构**:

- **容器化**: Docker
- **服务器**: 单台生产服务器 (SVRA)
- **网络**: 0.0.0.0 绑定（支持 Docker 端口映射）
- **端口映射**:
  - HTTP: 8080(host) → 9080(container)
  - HTTPS: 9444(host) → 9444(container)
  - gRPC: 9090(container内部)

**数据存储**:

- **MySQL**: RDS 托管服务
- **Redis**: Docker 容器

**监控与备份**:

- **健康检查**: 多层（Docker HEALTHCHECK + GitHub Actions）
- **自动备份**: 每天凌晨 01:00（保留 3 次）
- **自动恢复**: unhealthy 容器自动重启

---

## Secrets 配置

### 配置位置

1. **进入 Settings**
   - **Repository Secrets**: `Settings` → `Secrets and variables` → `Actions`
   - **Organization Secrets**: 组织设置 → `Secrets and variables` → `Actions`

2. **点击 `New repository secret` 或 `New organization secret`**

3. **添加以下 Secrets**

### 必需的 Secrets

#### Organization Secrets（组织级别，8个）

**服务器连接**:

| Secret 名称 | 说明 | 示例值 | 使用场景 |
| ------------ | ------ | -------- | --------- |
| `SVRA_HOST` | 生产服务器 IP/域名 | `192.168.1.100` | 所有 SSH 操作 |
| `SVRA_USERNAME` | SSH 登录用户名 | `deploy` | 所有 SSH 操作 |
| `SVRA_SSH_KEY` | SSH 私钥（完整） | 见下方 SSH 配置 | 所有 SSH 操作 |
| `SVRA_SSH_PORT` | SSH 端口 | `22` | 所有 SSH 操作 |

**数据库连接** (共享配置):

| Secret 名称 | 说明 | 示例值 | 使用场景 |
| ------------ | ------ | -------- | --------- |
| `MYSQL_HOST` | MySQL 服务器地址 | `192.168.1.101` | 应用运行、健康检查 |
| `MYSQL_PORT` | MySQL 端口 | `3306` | 应用运行、健康检查 |
| `REDIS_HOST` | Redis 服务器地址 | `localhost` | 应用运行、健康检查 |
| `REDIS_PORT` | Redis 端口 | `6379` | 应用运行、健康检查 |

#### Repository Secrets（仓库级别，5个）

**数据库凭证** (敏感信息):

| Secret 名称 | 说明 | 示例值 | 使用场景 |
| ------------ | ------ | -------- | --------- |
| `MYSQL_USERNAME` | MySQL 用户名 | `iam_user` | 应用、备份、健康检查 |
| `MYSQL_PASSWORD` | MySQL 密码 | `your_secure_password` | 应用、备份、健康检查 |
| `MYSQL_DBNAME` | 数据库名称 | `iam_db` | 应用、备份、健康检查 |
| `REDIS_PASSWORD` | Redis 密码 | `your_redis_password` | 应用、健康检查 |
| `REDIS_DB` | Redis 数据库编号 | `0` | 应用配置 |

### ✅ 验证配置

配置完成后，**强烈建议**按以下顺序验证：

#### 1. SSH 连接和时区验证

```bash
Actions → Test SSH Connection → Run workflow
```

验证内容：

- ✅ SSH 连接成功
- ✅ 服务器时区正确（Asia/Shanghai）
- ✅ UTC 时间与本地时间转换正确
- ✅ Docker 和 IAM 服务运行正常

**预期输出示例**：

```text
Time Information:
  Local Time: 2023-10-24 15:30:00 CST  ← 北京时间
  UTC Time: 2023-10-24 07:30:00 UTC    ← UTC 时间
Timezone Configuration:
  Time zone: Asia/Shanghai (CST, +0800)
```

#### 2. 快速连通性检查

```bash
Actions → Ping Runner → Run workflow
```

验证内容：

- ✅ 生产服务器响应
- ✅ 系统资源正常
- ✅ Docker 服务运行
- ✅ API 端点可访问

#### 3. 数据库状态检查

```bash
Actions → Database Operations → Run workflow → 选择 "status"
```

验证内容：

- ✅ MySQL 连接成功
- ✅ 数据库可访问
- ✅ 表结构正常
- ✅ 备份目录存在

#### 4. 完整健康检查

```bash
Actions → Server Health Check → Run workflow
```

验证内容：

- ✅ 系统健康（CPU、内存、磁盘）
- ✅ Docker 服务正常
- ✅ IAM 容器健康
- ✅ 网络和 API 正常
- ✅ 数据库和 Redis 连接正常

#### 5. 测试自动备份（可选）

等待自动备份运行（北京时间凌晨 01:00），或手动触发：

```bash
Actions → Database Operations → Run workflow → 选择 "backup"
```

然后检查：

```bash
Actions → Database Operations → Run workflow → 选择 "status"
# 查看 "Available backups" 部分
```

---

## 使用指南

### 日常开发流程

#### 1. 功能开发（develop 分支）

```bash
# 创建功能分支
git checkout -b feature/user-management develop

# 开发并本地测试
make test
make lint
make build

# 提交代码
git add .
git commit -m "feat: add user management feature"
git push origin feature/user-management

# 创建 PR 到 develop 分支
# GitHub 自动运行: test + lint
```

#### 2. 发布到生产（main 分支）

```bash
# 合并 develop 到 main
git checkout main
git merge develop
git push origin main

# 自动触发完整 CI/CD 流程:
# 1. Validate Secrets
# 2. Test + Lint (并行)
# 3. Build
# 4. Docker Build & Push
# 5. Deploy to Production
# 6. Health Check
```

#### 3. 紧急修复（hotfix）

```bash
# 从 main 创建 hotfix 分支
git checkout -b hotfix/critical-bug main

# 修复并测试
make test

# 提交并合并回 main
git add .
git commit -m "fix: resolve critical security issue"
git push origin hotfix/critical-bug

# 创建 PR 到 main，快速审查后合并
# 自动触发部署
```

### 数据库管理

#### 自动备份

- **时间**: 每天北京时间凌晨 01:00
- **保留**: 最近 3 次备份
- **位置**: `/opt/backups/iam/database/`
- **无需手动干预**

#### 手动备份

```bash
# 重要操作前建议手动备份
Actions → Database Operations → Run workflow → 选择 "backup"
```

#### 恢复数据库

```bash
# 1. 查看可用备份
Actions → Database Operations → Run workflow → 选择 "status"

# 2. 记录要恢复的备份文件名
# 例如: iam_backup_20231024_010000.sql.gz

# 3. 执行恢复
Actions → Database Operations → Run workflow → 选择 "restore"
# 输入备份文件名: iam_backup_20231024_010000.sql.gz

# ⚠️ 注意: 5 秒延迟给你反悔的机会
```

#### 数据库迁移

```bash
# 在容器内运行迁移
Actions → Database Operations → Run workflow → 选择 "migrate"

# 迁移会自动在以下情况执行:
# 1. 每次部署时（cicd.yml）
# 2. 应用启动时（如果配置了）
```

### 监控和告警

#### 查看工作流状态

访问: `https://github.com/FangcunMount/iam/v2/actions`

**自动监控时间表**:

- ⏰ **01:00** (北京时间) - 数据库自动备份
- ⏰ **每 30 分钟** - 服务器健康检查
- ⏰ **每 6 小时** - 快速连通性检查

#### 添加状态徽章（可选）

在项目 `README.md` 中添加：

```markdown
![CI/CD](https://github.com/FangcunMount/iam/v2/workflows/CI/CD%20Pipeline/badge.svg)
![Health](https://github.com/FangcunMount/iam/v2/workflows/Server%20Health%20Check/badge.svg)
![Ping](https://github.com/FangcunMount/iam/v2/workflows/Ping%20Runner/badge.svg)
```

#### GitHub Actions 通知设置

1. 进入 `Settings` → `Notifications`
2. 勾选 `Actions`
3. 选择通知方式：
   - Email（推荐：仅失败时通知）
   - Web
   - Mobile

---

## 故障排查

### 常见问题

#### 1. SSH 连接失败

**错误信息**: `Permission denied (publickey)`

**排查步骤**:

```bash
# 1. 验证 SSH 配置
Actions → Test SSH Connection → Run workflow
# 查看详细错误信息

# 2. 检查 Secrets 是否正确配置
Settings → Secrets → 确认 SVRA_* 存在

# 3. 测试本地 SSH 连接
ssh -i ~/.ssh/your_key user@server-host

# 4. 检查服务器 authorized_keys
ssh user@server "cat ~/.ssh/authorized_keys"

# 5. 检查服务器 SSH 日志
ssh user@server "sudo journalctl -u ssh -n 50"
```

**解决方案**:

- 确保私钥格式正确（包括 BEGIN/END 行）
- 验证公钥在服务器 `~/.ssh/authorized_keys` 中
- 检查文件权限: `chmod 600 ~/.ssh/authorized_keys`
- 确认 SSH 配置允许公钥认证: `PubkeyAuthentication yes`

#### 2. 部署失败 - 健康检查超时

**症状**: 部署显示 "Health check failed after 150 seconds"

**排查步骤**:

```bash
# 1. 检查容器状态
Actions → Ping Runner → Run workflow
# 或
Actions → Server Health Check → Run workflow

# 2. SSH 登录查看容器日志
ssh user@server
sudo docker ps -a | grep iam-apiserver
sudo docker logs --tail 100 iam-apiserver

# 3. 检查端口绑定
sudo docker port iam-apiserver
sudo netstat -tlnp | grep -E "8080|9080|9444"

# 4. 手动测试 API
curl http://localhost:8080/healthz
curl -k https://localhost:9444/healthz
```

**常见原因**:

- ❌ 配置文件错误（apiserver.yaml）
- ❌ 数据库连接失败
- ❌ 端口被占用
- ❌ 容器内存不足

**解决方案**:

```bash
# 检查配置
sudo docker exec iam-apiserver cat /opt/iam/configs/apiserver.yaml

# 重启容器
sudo docker restart iam-apiserver

# 查看详细日志
sudo docker logs --tail 200 iam-apiserver

# 如果需要回滚
cd /opt/backups/iam/deployments
# 找到最近的备份并恢复
```

#### 3. 数据库连接失败

**错误信息**: `Access denied` 或 `Can't connect to MySQL server`

**排查步骤**:

```bash
# 1. 验证数据库配置
Actions → Database Operations → Run workflow → "status"

# 2. 测试数据库连接
ssh user@server
mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USERNAME -p

# 3. 检查数据库用户权限
mysql -u root -p
> SELECT user, host FROM mysql.user WHERE user='iam_user';
> SHOW GRANTS FOR 'iam_user'@'%';

# 4. 检查网络连接
ping $MYSQL_HOST
telnet $MYSQL_HOST 3306
```

**解决方案**:

- 确认 Secrets 中的数据库凭证正确
- 检查 RDS 安全组规则（允许服务器 IP）
- 验证数据库用户权限
- 检查数据库防火墙规则

#### 4. Docker 容器 unhealthy

**症状**: 容器状态显示 `(unhealthy)`

**自动恢复**:

- `server-check.yml` 会自动检测并重启 unhealthy 容器

**手动排查**:

```bash
# 1. 查看健康检查日志
sudo docker inspect --format='{{json .State.Health}}' iam-apiserver | jq

# 2. 手动执行健康检查命令
sudo docker exec iam-apiserver curl -f http://localhost:9080/healthz

# 3. 查看应用日志
sudo docker logs --tail 100 iam-apiserver

# 4. 检查资源使用
sudo docker stats iam-apiserver --no-stream
```

**常见原因**:

- `/healthz` 端点返回非 200 状态
- 应用启动时间过长（超过 30 秒 start-period）
- 内存不足导致应用崩溃
- 数据库连接池耗尽

#### 5. 自动备份失败

**症状**: `db-ops.yml` workflow 失败

**排查步骤**:

```bash
# 1. 查看工作流日志
Actions → Database Operations → 查看失败的运行

# 2. 检查备份目录
ssh user@server
ls -lh /opt/backups/iam/database/
df -h  # 检查磁盘空间

# 3. 手动执行备份命令
mysqldump -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USERNAME -p \
  --single-transaction --routines --triggers $MYSQL_DBNAME > test_backup.sql
```

**解决方案**:

- 确保 `/opt/backups/iam/database/` 目录存在且有写权限
- 检查磁盘空间是否充足
- 验证数据库凭证正确
- 检查 mysqldump 命令是否安装

#### 6. 时区问题 - 备份时间不对

**症状**: 备份没有在预期时间执行

**验证步骤**:

```bash
# 1. 检查服务器时区
Actions → Test SSH Connection → Run workflow
# 查看 "Time Information" 和 "Timezone Configuration"

# 2. 验证 cron 表达式
# GitHub Actions 使用 UTC 时间
# 0 17 * * * (UTC 17:00) = 北京时间 01:00

# 3. 查看最近的备份时间
Actions → Database Operations → "status"
# 查看备份文件的时间戳
```

**时区转换参考**:

```text
北京时间 = UTC + 8 小时

想要的北京时间 → UTC cron
01:00 → 0 17 * * *  (17:00 UTC)
02:00 → 0 18 * * *  (18:00 UTC)
03:00 → 0 19 * * *  (19:00 UTC)
```

#### 7. 回滚到之前版本

**快速回滚**:

```bash
# 1. SSH 登录服务器
ssh user@server

# 2. 查看可用备份
ls -lht /opt/backups/iam/deployments/ | head -6

# 3. 选择要回滚的版本
BACKUP_DIR="/opt/backups/iam/deployments/backup_20231024_100000"

# 4. 停止当前服务
sudo docker stop iam-apiserver
sudo docker rm iam-apiserver

# 5. 使用备份的镜像
cd $BACKUP_DIR
# 查看备份信息
cat deployment_info.txt

# 6. 拉取特定版本镜像（如果有 image ID）
sudo docker pull ghcr.io/fangcunmount/iam:specific-tag

# 7. 启动容器
sudo docker run -d \
  --name iam-apiserver \
  -p 8080:9080 \
  -p 9444:9444 \
  --restart unless-stopped \
  ghcr.io/fangcunmount/iam:specific-tag

# 8. 验证服务
curl http://localhost:8080/healthz
```

**数据库回滚**:

```bash
# 如果需要恢复数据库
Actions → Database Operations → "restore"
# 选择对应时间的备份
```

---

## 最佳实践

### SSH 密钥配置指南

#### 1. 生成 SSH 密钥对

```bash
# 在本地生成密钥（推荐使用 ed25519 算法）
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/github_actions_deploy

# 会生成两个文件:
# ~/.ssh/github_actions_deploy      (私钥) ← 用于 GitHub Secrets
# ~/.ssh/github_actions_deploy.pub  (公钥) ← 用于服务器
```

#### 2. 配置生产服务器

```bash
# 方法 1: 使用 ssh-copy-id (推荐)
ssh-copy-id -i ~/.ssh/github_actions_deploy.pub user@svra-host

# 方法 2: 手动添加
cat ~/.ssh/github_actions_deploy.pub | ssh user@svra-host "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"

# 在服务器上设置正确权限
ssh user@svra-host "chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"
```

#### 3. 添加私钥到 GitHub Secrets

```bash
# 1. 复制私钥内容
cat ~/.ssh/github_actions_deploy
# 或使用 pbcopy (macOS)
cat ~/.ssh/github_actions_deploy | pbcopy

# 2. 在 GitHub 上添加:
# Settings → Secrets → New organization secret
# Name: SVRA_SSH_KEY
# Value: 粘贴完整的私钥内容（必须包括 -----BEGIN 和 -----END 行）
```

#### 4. 测试 SSH 连接

```bash
# 本地测试
ssh -i ~/.ssh/github_actions_deploy user@svra-host

# GitHub Actions 测试
Actions → Test SSH Connection → Run workflow
```

#### 5. 安全建议

- ✅ 使用 ed25519 算法（比 RSA 更安全更快）
- ✅ 为密钥添加有意义的注释（-C 参数）
- ✅ 定期轮换密钥（建议每 3-6 个月）
- ✅ 限制密钥用途（仅用于 CI/CD）
- ✅ 不要复用个人 SSH 密钥
- ❌ 不要在私钥上设置密码（GitHub Actions 无法交互输入）

### 开发流程最佳实践

### 首次部署

#### 1. 配置生产服务器

```bash
# SSH 登录到 SVRA
ssh user@svra-host

# 创建必要目录
sudo mkdir -p /opt/iam
sudo mkdir -p /opt/backups/iam
sudo chown -R $USER:$USER /opt/iam /opt/backups/iam

# 克隆仓库
cd /opt
git clone https://github.com/FangcunMount/iam/v2.git
cd iam

# 安装依赖
go mod download
```

#### 2. 配置 systemd 服务

```bash
# 复制服务文件
sudo cp build/systemd/iam-apiserver.service /etc/systemd/system/

# 重载 systemd
sudo systemctl daemon-reload

# 启用服务
sudo systemctl enable iam-apiserver

# 启动服务
sudo systemctl start iam-apiserver

# 检查状态
sudo systemctl status iam-apiserver
```

#### 3. 配置应用

```bash
# 编辑配置文件
vim configs/apiserver.yaml

# 或使用环境变量
vim configs/env/config.prod.env
```

### 日常使用

#### 自动部署（推荐）

```bash
# 提交代码到 develop 分支（测试）
git checkout develop
git add .
git commit -m "feat: add new feature"
git push origin develop

# 合并到 main 分支（生产部署）
git checkout main
git merge develop
git push origin main
# 自动触发 CI/CD → 测试 → 构建 → 部署
```

#### 手动触发部署

```bash
# 在 GitHub 页面
Actions → CI/CD Pipeline → Run workflow → 选择分支 → Run
```

#### 数据库备份

```bash
# 方式1: GitHub Actions（推荐）
Actions → Database Operations → Run workflow → 选择 "backup"

# 方式2: 服务器上手动备份
ssh user@svra-host
mysqldump -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USERNAME -p$MYSQL_PASSWORD $MYSQL_DBNAME > backup.sql
gzip backup.sql
```

#### 数据库恢复

```bash
# 1. 查看可用备份
Actions → Database Operations → Run workflow → 选择 "status"

# 2. 恢复指定备份
Actions → Database Operations → Run workflow → 选择 "restore"
# 输入备份文件名: iam_backup_20231022_120000.sql.gz
```

#### 查看服务状态

```bash
# 方式1: GitHub Actions
Actions → Server Health Check → Run workflow

# 方式2: SSH 登录查看
ssh user@svra-host
systemctl status iam-apiserver
journalctl -u iam-apiserver -f
```

### 监控和告警

#### 查看工作流状态

- 访问: `https://github.com/FangcunMount/iam/v2/actions`
- 每个工作流执行都有详细日志

#### 自动健康检查时间表

- **Runner 连通性**: 每6小时自动检查
- **服务器健康**: 每30分钟自动检查
- **部署验证**: 每次部署后自动验证

#### 添加状态徽章

在项目 README.md 中添加：

```markdown
![CI/CD](https://github.com/FangcunMount/iam/v2/workflows/CI/CD%20Pipeline/badge.svg)
![Health](https://github.com/FangcunMount/iam/v2/workflows/Server%20Health%20Check/badge.svg)
![Ping](https://github.com/FangcunMount/iam/v2/workflows/Ping%20Runner/badge.svg)
```

---

## 附加资源

### 项目文档

- [架构概览](../../docs/architecture-overview.md)
- [部署检查清单](../../docs/DEPLOYMENT_CHECKLIST.md)
- [认证文档](../../docs/authn/README.md)
- [授权文档](../../docs/authz/README.md)

### 外部资源

- [GitHub Actions 官方文档](https://docs.github.com/en/actions)
- [GitHub Secrets 安全指南](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- [Docker 最佳实践](https://docs.docker.com/develop/dev-best-practices/)
- [Conventional Commits](https://www.conventionalcommits.org/)

### 命令行工具

```bash
# GitHub CLI
brew install gh
gh workflow list                    # 列出所有工作流
gh run list --workflow=cicd.yml    # 查看特定工作流运行
gh run view <run-id> --log         # 查看运行日志

# Docker
docker ps                           # 查看运行中的容器
docker logs iam-apiserver           # 查看容器日志
docker stats iam-apiserver          # 查看资源使用
docker system prune -a              # 清理未使用的资源
```

---

## 快速参考

### 常用操作

```bash
# 部署到生产
git push origin main

# 手动备份数据库
Actions → Database Operations → backup

# 查看数据库状态
Actions → Database Operations → status

# 健康检查
Actions → Server Health Check → Run workflow

# SSH 连接测试
Actions → Test SSH Connection → Run workflow

# 查看容器日志
ssh user@svra "sudo docker logs --tail 100 iam-apiserver"
```

### Secrets 清单

**Organization Secrets (8个)**:

```text
SVRA_HOST, SVRA_USERNAME, SVRA_SSH_KEY, SVRA_SSH_PORT
MYSQL_HOST, MYSQL_PORT, REDIS_HOST, REDIS_PORT
```

**Repository Secrets (5个)**:

```text
MYSQL_USERNAME, MYSQL_PASSWORD, MYSQL_DBNAME
REDIS_PASSWORD, REDIS_DB
```

### 时区转换参考

GitHub Actions cron 使用 **UTC 时间**：

| 北京时间 | UTC 时间 | Cron 表达式 |
| --------- | --------- | ------------ |
| 01:00 | 17:00 (前一天) | `0 17 * * *` |
| 02:00 | 18:00 (前一天) | `0 18 * * *` |
| 03:00 | 19:00 (前一天) | `0 19 * * *` |
| 10:00 | 02:00 | `0 2 * * *` |

---

**最后更新**: 2025年10月23日

**维护**: FangcunMount Team

**支持**: 通过 GitHub Issues 提交问题或建议

**✅ Organization Secrets (8个)**:
SVRA_HOST, SVRA_USERNAME, SVRA_SSH_KEY, SVRA_SSH_PORT,
MYSQL_HOST, MYSQL_PORT, REDIS_HOST, REDIS_PORT

**✅ Repository Secrets (5个)**:
MYSQL_USERNAME, MYSQL_PASSWORD, MYSQL_DBNAME,
REDIS_PASSWORD, REDIS_DB

---

## 获取帮助

### 问题排查顺序

1. **查看 GitHub Actions 日志** - 最详细的错误信息
2. **检查服务器日志** - `journalctl -u iam-apiserver`
3. **验证 Secrets 配置** - 确保所有 Secrets 正确配置
4. **测试连通性** - 运行 ping-runner 工作流
5. **查看本文档** - 查找相关故障排查步骤

### 支持渠道

- **GitHub Issues**: 提交问题和功能请求
- **Pull Requests**: 提交改进和修复
- **文档**: 查阅项目文档目录

---

**最后更新**: 2025年10月22日

**环境**: 开发环境（MacBook）+ 生产环境（SVRA 服务器 A）

**CI/CD**: GitHub Actions + Docker + systemd
