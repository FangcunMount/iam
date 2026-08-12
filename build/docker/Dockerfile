# ============================================================================
# 多阶段构建 Dockerfile for IAM
# ============================================================================

# ============================================================================
# 构建阶段
# ============================================================================
FROM golang:1.25.9-alpine AS builder

# 设置工作目录
WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git make

# 设置 Go 环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN go build \
    -ldflags "-s -w \
        -X github.com/FangcunMount/iam/v2/pkg/version.GitVersion=${VERSION} \
        -X github.com/FangcunMount/iam/v2/pkg/version.BuildDate=${BUILD_TIME} \
        -X github.com/FangcunMount/iam/v2/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o apiserver \
    ./cmd/apiserver/

RUN go build \
    -ldflags "-s -w" \
    -o iam-maintenance \
    ./cmd/iam-maintenance/

# ============================================================================
# 运行阶段
# ============================================================================
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户，与宿主机 www UID/GID 对齐，由 CI build-arg 传入
ARG RUN_UID=2000
ARG RUN_GID=2000
RUN addgroup -g ${RUN_GID} www && \
    adduser -D -u ${RUN_UID} -G www www

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/apiserver /app/apiserver
COPY --from=builder /build/iam-maintenance /app/iam-maintenance

# 复制配置文件
COPY --chown=www:www configs /app/configs

# 创建应用运行所需目录（日志、持久化密钥等）
RUN mkdir -p /app/logs /app/data/keys /var/log/iam && \
    chown -R www:www /app /var/log/iam

# 预创建 TLS 目录，避免按文件挂载失败
RUN mkdir -p /etc/iam/ssl /etc/iam/grpc/ca /etc/iam/grpc/server && \
    chmod 755 /etc/iam /etc/iam/ssl /etc/iam/grpc /etc/iam/grpc/ca /etc/iam/grpc/server

# 切换到非 root 用户
USER www

# 暴露端口
# 注意：EXPOSE 仅为文档声明，实际端口映射由 docker-compose 或 k8s 控制
#
# REST API 端口（主要服务入口）
# HTTP REST API (内网服务调用，无 TLS 开销，性能更好)
EXPOSE 9080  
# HTTPS REST API (外网访问，TLS 加密保护)
EXPOSE 9444  
#
# gRPC 端口（服务间通信，mTLS 双向认证保护）
# gRPC 服务 (需要客户端证书，仅限内网可信服务)
EXPOSE 9090  
# gRPC 健康检查 (HTTP，供监控系统使用)
EXPOSE 9091  

# 健康检查 (检查实际监听的端口 9080)
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:9080/healthz || exit 1

# 启动命令
ENTRYPOINT ["/app/apiserver"]
CMD ["--config=/app/configs/apiserver.prod.yaml"]
