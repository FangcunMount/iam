# 环境配置说明

本文档说明如何配置 iam contracts 的环境变量。

## 配置文件

项目包含以下环境配置文件：

- `config.dev.env` - 开发环境配置
- `config.prod.env` - 生产环境配置

## 配置项说明

### 运行模式

- YAML 使用 `server.mode`。
- 环境变量覆盖使用 `IAM_APISERVER_SERVER_MODE`。
- 启动参数覆盖使用 `--server.mode`。
- 只接受 `debug`、`test`、`release`，默认值为 `release`。
- 显式启动参数优先于环境变量，环境变量优先于 YAML。
- 生产部署包默认由 `apiserver.prod.yaml` 固定为 `release`，不在生成的 Secret env 文件中重复写入。

`app.*`、`server.run-mode`、`server.name`、`server.read-timeout` 和 `server.write-timeout` 已删除；配置文件或对应环境变量仍设置这些键时，iam-apiserver 会拒绝启动。

### MySQL 配置

- **MYSQL_ROOT_PASSWORD**: MySQL root 用户密码
- **MYSQL_DATABASE**: 数据库名称
- **MYSQL_USER**: 应用用户名称
- **MYSQL_PASSWORD**: 应用用户密码
- **MYSQL_PORT**: MySQL 服务端口
- **MYSQL_CONTAINER_NAME**: Docker 容器名称
- **MYSQL_IMAGE_NAME**: Docker 镜像名称

### Redis 配置

- **REDIS_PASSWORD**: Redis 密码
- **REDIS_PORT**: Redis 服务端口
- **REDIS_CONTAINER_NAME**: Docker 容器名称
- **REDIS_IMAGE_NAME**: Docker 镜像名称

### Docker 配置

- **DOCKER_NETWORK_NAME**: Docker 网络名称
- **TZ**: 时区设置

### 数据路径配置

- **MYSQL_DATA_PATH**: MySQL 数据目录
- **MYSQL_LOGS_PATH**: MySQL 日志目录
- **REDIS_DATA_PATH**: Redis 数据目录
- **REDIS_LOGS_PATH**: Redis 日志目录

## 使用方法

### 开发环境

1. 复制配置文件：

```bash
cp config.dev.env config.local.env
```

2.根据需要修改配置项

3.启动服务时使用配置文件：

```bash
docker-compose --env-file config.dev.env up -d
docker-compose --env-file config.local.env up -d
```

### 生产环境

1.复制生产环境配置：

```bash
cp config.prod.env config.production.env
```

2.修改敏感信息（密码等）

3.启动服务：

```bash
docker-compose --env-file config.production.env up -d
```

## 安全注意事项

1. **密码安全**: 生产环境请使用强密码
2. **文件权限**: 确保配置文件权限正确
3. **网络安全**: 生产环境建议使用内部网络
4. **备份策略**: 定期备份数据库和配置文件

## 故障排除

### 常见问题

1. **连接失败**: 检查端口和密码配置
2. **权限错误**: 检查数据目录权限
3. **容器启动失败**: 检查环境变量配置

### 调试方法

```bash
# 查看环境变量
docker-compose config

# 查看容器日志
docker-compose logs [service-name]

# 进入容器调试
docker exec -it [container-name] bash
```
