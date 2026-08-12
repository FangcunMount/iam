# Docker 构建文件

本目录包含 Docker 相关的构建和部署文件。

## 文件说明

### Dockerfile
多阶段构建的 Docker 镜像配置：
- **构建阶段**：使用 golang:1.23-alpine 编译 Go 应用
- **运行阶段**：使用 alpine:3.19 最小化镜像
- **特性**：
  - 非 root 用户运行（安全）
  - 健康检查配置
  - 时区设置（Asia/Shanghai）
  - 多架构支持（linux/amd64）

### docker-compose.prod.yml
Docker Compose 编排配置，包含：
- **iam-apiserver**：IAM API 服务器（加入外部 `infra-network`，依赖云 RDS / Redis-Store 和网关；`image` 使用外部仓库，生产不在目标机构建）

适合生产部署或接入现有基础设施（不再内置 MySQL/Redis/Nginx）。

### .dockerignore
Docker 构建忽略文件，减小构建上下文大小：
- 排除构建产物（bin/, tmp/, logs/）
- 排除 IDE 文件（.vscode/, .idea/）
- 排除文档和配置文件
- 排除测试覆盖率文件

## 使用方法

### 方式 1：使用 Makefile（推荐）

```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run

# 使用 Docker Compose 启动所有服务
make docker-compose-up

# 查看日志
make docker-compose-logs

# 停止服务
make docker-compose-down

# 清理镜像和容器
make docker-clean
```

### 方式 2：直接使用 Docker 命令

```bash
# 构建镜像
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S') \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -f build/docker/Dockerfile \
  -t iam:latest \
  .

# 运行容器
docker run -d \
  --name iam-apiserver \
  --network infra-network \
  -p 9080:9080 \  # HTTP REST API (内网)
  -p 9444:9444 \  # HTTPS REST API (外网)
  -v $(pwd)/configs:/app/configs:ro \
  -v $(pwd)/logs:/app/logs \
  iam:latest

# 查看日志
docker logs -f iam-apiserver

# 停止容器
docker stop iam-apiserver
docker rm iam-apiserver
```

### 方式 3：使用 Docker Compose

```bash
# 启动所有服务
docker-compose -f build/docker/docker-compose.prod.yml up -d

# 查看状态
docker-compose -f build/docker/docker-compose.prod.yml ps

# 查看日志
docker-compose -f build/docker/docker-compose.prod.yml logs -f iam-apiserver

# 停止服务
docker-compose -f build/docker/docker-compose.prod.yml down

# 重启服务
docker-compose -f build/docker/docker-compose.prod.yml restart iam-apiserver
```

## 环境变量配置

可以通过环境变量覆盖配置：

```bash
# 方式 1：命令行传递
docker run -d \
  -e MYSQL_HOST=your-db-host \
  -e MYSQL_PORT=3306 \
  -e REDIS_CACHE_HOST=redis-cache \
  -e REDIS_CACHE_PORT=6379 \
  -e REDIS_STORE_HOST=your-redis.redis.rds.aliyuncs.com \
  -e REDIS_STORE_PORT=6379 \
  iam:latest

# 方式 2：使用 .env 文件（Docker Compose）
# 创建 .env 文件
cat > .env <<EOF
MYSQL_HOST=your-rds.mysql.rds.aliyuncs.com
MYSQL_PORT=3306
MYSQL_DATABASE=iam
MYSQL_USER=iam
MYSQL_PASSWORD=PLEASE_CHANGE_ME
REDIS_CACHE_HOST=redis-cache
REDIS_CACHE_PORT=6379
REDIS_STORE_HOST=your-redis.redis.rds.aliyuncs.com
REDIS_STORE_PORT=6379
EOF

# 启动（自动读取 .env 文件）
docker-compose -f build/docker/docker-compose.prod.yml up -d
```

## 健康检查

容器包含健康检查配置，会定期检查服务状态：

```bash
# 查看容器健康状态
docker inspect --format='{{.State.Health.Status}}' iam-apiserver

# 手动执行健康检查
docker exec iam-apiserver curl -f http://localhost:9080/healthz

# 检查实例是否可以接收流量
docker exec iam-apiserver curl -f http://localhost:9080/readyz
```

Docker `HEALTHCHECK` 始终使用 `/healthz` 作为 liveness。生产发布脚本会先
等待 `/healthz=200`，再等待 `/readyz=200`；readiness 未通过时发布失败，
但不会自动重启或回滚容器。

镜像同时包含 `/app/iam-maintenance`，仅用于受控维护窗口中的 Refresh
Token 清理、修复前敏感日志处置和旧 AuthN 表到 canonical 表的安全对账/补缺。
维护命令默认 dry-run，不由 apiserver 自动调用；AuthN apply 必须通过生产
Database Operations 的备份、审批和确认词门禁，且不会删除旧表。

## 日志管理

### 容器日志

```bash
# 查看实时日志
docker logs -f iam-apiserver

# 查看最近 100 行
docker logs --tail 100 iam-apiserver

# 查看带时间戳的日志
docker logs -t iam-apiserver
```

### 应用日志

应用日志会输出到挂载的日志目录：

```bash
# 查看应用日志
tail -f logs/apiserver.log
```

## 数据持久化

### 使用 Docker Volume

```bash
# 创建数据卷
docker volume create iam-data

# 运行容器并挂载数据卷
docker run -d \
  --name iam-apiserver \
  -v iam-data:/app/data \
  -v $(pwd)/configs:/app/configs:ro \
  iam:latest
```

### 使用主机目录

```bash
# 挂载主机目录
docker run -d \
  --name iam-apiserver \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs:ro \
  -v $(pwd)/logs:/app/logs \
  iam:latest
```

## 网络配置

### 使用自定义网络

```bash
# 创建 overlay 网络（需先 init swarm）
docker network create --driver overlay --attachable infra-network

# 运行容器并连接到网络
docker run -d \
  --name iam-apiserver \
  --network infra-network \
  iam:latest
```

Docker Compose 使用外部的 `infra-network`（需预先创建，不会自动创建）。

## 故障排查

### 容器无法启动

```bash
# 查看容器日志
docker logs iam-apiserver

# 查看容器详细信息
docker inspect iam-apiserver

# 进入容器调试
docker exec -it iam-apiserver sh
```

### 端口冲突

```bash
# 检查端口占用
lsof -i :9080
# 或
netstat -tlnp | grep 9080

# 使用不同端口
docker run -d -p 9180:9080 iam:latest
```

### 配置文件问题

```bash
# 验证配置文件挂载
docker exec iam-apiserver ls -la /app/configs

# 查看配置文件内容
docker exec iam-apiserver cat /app/configs/apiserver.prod.yaml

# 手动测试启动
docker exec -it iam-apiserver /app/apiserver --config=/app/configs/apiserver.prod.yaml
```

## 生产环境注意事项

1. **安全配置**：
   - 使用非 root 用户运行（已配置）
   - 限制容器资源（CPU、内存）
   - 定期更新基础镜像

2. **资源限制**：
   ```bash
   docker run -d \
     --cpus=2 \
     --memory=2g \
     --memory-swap=2g \
     iam:latest
   ```

3. **重启策略**：
   ```bash
   docker run -d \
     --restart=unless-stopped \
     iam:latest
   ```

4. **日志轮转**：
   ```bash
   docker run -d \
     --log-driver json-file \
     --log-opt max-size=10m \
     --log-opt max-file=3 \
     iam:latest
   ```

5. **监控和告警**：
   - 集成 Prometheus 监控
   - 配置健康检查告警
   - 监控容器资源使用

## 多架构支持

如需构建多架构镜像（如 ARM64）：

```bash
# 创建构建器
docker buildx create --use

# 构建多架构镜像
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v1.0.0 \
  -f build/docker/Dockerfile \
  -t iam:latest \
  --push \
  .
```

## 相关文档

- [部署总览](../../docs/DEPLOYMENT.md) - 所有部署方式说明
- [Jenkins 部署](../../docs/JENKINS_QUICKSTART.md) - CI/CD 自动化部署
- [主 README](../../README.md) - 项目概述

## 生产环境部署清单

部署前请确保在目标服务器上创建以下目录和文件：

### 1. 运行用户 UID/GID

`docker-compose.prod.yml` 中服务以 `user: WWW_UID:WWW_GID` 运行，且生产部署要求显式提供这两个变量；镜像内默认运行用户为 `www`，本地默认 UID/GID 为 `2000:2000`。CI 部署脚本会按 `WWW_UID`/`WWW_GID` 对配置、日志和密钥目录做 `chown`；手动部署时请自行对齐。

公网 HTTPS 证书由 `serverA` 上的 Nginx 终止，`serverB` 上的 IAM 容器不再挂载 `/data/ssl/certs` 和 `/data/ssl/private`。

### 2. gRPC mTLS 证书

```bash
# CA 证书链
/data/infra/ssl/grpc/ca/ca-chain.crt

# 服务端证书
/data/infra/ssl/grpc/server/iam-apiserver.crt
/data/infra/ssl/grpc/server/iam-apiserver.key
```

### 3. 其他目录

```bash
# 日志目录
mkdir -p /data/logs/iam

# JWKS 密钥目录
mkdir -p /data/ops/iam-keys
```

> **注意**：gRPC ACL 配置文件 (`grpc_acl.yaml`) 已包含在项目 `configs/` 目录中，
> 通过 docker-compose 挂载到容器的 `/app/configs/` 路径，无需单独部署。

### 完整初始化脚本

```bash
#!/bin/bash
# 生产环境初始化脚本

# 创建目录结构
mkdir -p /data/infra/ssl/grpc/ca
mkdir -p /data/infra/ssl/grpc/server
mkdir -p /data/ops/iam-keys
mkdir -p /data/logs/iam

# 设置权限
chmod 755 /data/infra /data/infra/ssl /data/infra/ssl/grpc /data/infra/ssl/grpc/ca /data/infra/ssl/grpc/server
chmod 755 /data/ops /data/logs
chmod 750 /data/ops/iam-keys

# 提示：需要手动复制以下文件
echo "请手动复制以下文件："
echo "  - gRPC CA 证书到 /data/infra/ssl/grpc/ca/"
echo "  - gRPC 服务端证书到 /data/infra/ssl/grpc/server/"
```

## 技术支持

如有问题，请参考：
- [故障排查指南](../../docs/DEPLOYMENT.md#故障排查)
- [GitHub Issues](https://github.com/FangcunMount/iam/issues)
