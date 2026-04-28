#!/bin/bash

# ============================================================================
# IAM - 快速启动开发环境脚本
# ============================================================================
# 用途: 一键配置并启动本地开发环境
# 使用: ./scripts/quick-start-dev.sh
# ============================================================================

set -e  # 遇到错误立即退出

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印函数
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

# ============================================================================
# 1. 环境检查
# ============================================================================

print_header "步骤 1/7: 环境检查"

# 检查 Go
print_info "检查 Go 版本..."
if ! command -v go &> /dev/null; then
    print_error "Go 未安装！请先安装 Go 1.21+"
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
print_success "Go 已安装: $GO_VERSION"

# 检查 Make
print_info "检查 Make..."
if ! command -v make &> /dev/null; then
    print_error "Make 未安装！请先安装 Make"
    exit 1
fi
print_success "Make 已安装"

# 检查 Docker
print_info "检查 Docker..."
if ! command -v docker &> /dev/null; then
    print_warning "Docker 未安装，将使用本地 MySQL/Redis"
    USE_DOCKER=false
else
    print_success "Docker 已安装"
    USE_DOCKER=true
fi

# ============================================================================
# 2. 检查数据库服务
# ============================================================================

print_header "步骤 2/7: 检查数据库服务"

# 检查 MySQL
MYSQL_RUNNING=false
if lsof -i :3306 &> /dev/null; then
    print_success "MySQL 正在运行 (端口 3306)"
    MYSQL_RUNNING=true
elif docker ps | grep mysql &> /dev/null; then
    print_warning "MySQL 容器正在运行但没有端口映射"
    print_info "尝试重启 MySQL 容器并映射端口..."
    docker stop mysql 2>/dev/null || true
    docker rm mysql 2>/dev/null || true
fi

if [ "$MYSQL_RUNNING" = false ] && [ "$USE_DOCKER" = true ]; then
    print_info "启动 MySQL Docker 容器..."
    docker run -d \
        --name mysql \
        -p 3306:3306 \
        -e MYSQL_ROOT_PASSWORD=root \
        -e MYSQL_DATABASE=iam \
        -e MYSQL_USER=iam \
        -e MYSQL_PASSWORD=iam123 \
        mysql:8.0 \
        --character-set-server=utf8mb4 \
        --collation-server=utf8mb4_unicode_ci
    
    print_info "等待 MySQL 启动..."
    sleep 15
    print_success "MySQL 容器已启动"
fi

# 检查 Redis
REDIS_RUNNING=false
if lsof -i :6379 &> /dev/null; then
    print_success "Redis 正在运行 (端口 6379)"
    REDIS_RUNNING=true
fi

if [ "$REDIS_RUNNING" = false ] && [ "$USE_DOCKER" = true ]; then
    # 检查是否有其他 Redis 容器
    if docker ps | grep redis | grep 6379 &> /dev/null; then
        print_success "Redis 容器已在运行"
    else
        print_info "启动 Redis Docker 容器..."
        docker run -d \
            --name redis-dev \
            -p 6379:6379 \
            redis:7-alpine
        print_success "Redis 容器已启动"
    fi
fi

# ============================================================================
# 3. 下载依赖
# ============================================================================

print_header "步骤 3/7: 下载 Go 依赖"

print_info "下载项目依赖..."
go mod download
print_success "依赖下载完成"

# ============================================================================
# 4. 安装开发工具
# ============================================================================

print_header "步骤 4/7: 安装开发工具"

# 检查 Air
if ! command -v air &> /dev/null; then
    print_info "安装 Air (热重载工具)..."
    go install github.com/air-verse/air@latest
    print_success "Air 安装完成"
else
    print_success "Air 已安装"
fi

# ============================================================================
# 5. 生成开发证书
# ============================================================================

print_header "步骤 5/7: 生成开发证书"

if [ ! -f "configs/cert/web-apiserver.crt" ]; then
    print_info "生成自签名 SSL 证书..."
    mkdir -p configs/cert
    
    # 生成证书
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout configs/cert/web-apiserver.key \
        -out configs/cert/web-apiserver.crt \
        -subj "/C=CN/ST=Beijing/L=Beijing/O=IAM/OU=Dev/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
        2>/dev/null
    
    print_success "证书生成完成"
else
    print_success "证书已存在"
fi

# ============================================================================
# 6. 配置文件检查
# ============================================================================

print_header "步骤 6/7: 检查配置文件"

CONFIG_FILE="configs/apiserver.dev.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    print_error "配置文件不存在: $CONFIG_FILE"
    exit 1
fi

print_info "配置文件信息:"
echo "  - 配置文件: $CONFIG_FILE"
echo "  - HTTP 端口: 18081"
echo "  - HTTPS 端口: 18441"
echo "  - MySQL: 127.0.0.1:3306"
echo "  - Redis: 127.0.0.1:6379"
echo "  - 数据库: iam"
print_success "配置文件检查完成"

# ============================================================================
# 7. 提示信息
# ============================================================================

print_header "步骤 7/7: 环境准备完成"

print_success "开发环境准备完成！"
echo ""
print_info "数据库连接信息:"
echo "  MySQL:"
echo "    - 主机: 127.0.0.1:3306"
echo "    - 数据库: iam"
echo "    - 用户: root"
echo "    - 密码: root"
echo ""
echo "  Redis:"
echo "    - 主机: 127.0.0.1:6379"
echo "    - 密码: (无)"
echo ""
print_info "下一步操作:"
echo ""
echo "  1. 启动开发环境（热重载）:"
echo -e "     ${GREEN}make dev${NC}"
echo ""
echo "  2. 或者先构建再运行:"
echo -e "     ${GREEN}make build${NC}"
echo -e "     ${GREEN}make run${NC}"
echo ""
echo "  3. 查看日志:"
echo -e "     ${GREEN}make logs${NC}"
echo ""
echo "  4. 如需重放最小系统初始化数据:"
echo -e "     ${GREEN}make db-bootstrap${NC}"
echo ""
echo "  5. 健康检查:"
echo -e "     ${GREEN}curl http://localhost:18081/healthz${NC}"
echo ""
echo "  6. API 文档:"
echo -e "     ${GREEN}http://localhost:18081/swagger/index.html${NC}"
echo ""
print_info "提示: 第一次启动时，应用会自动创建数据库表结构"
print_info "首次启动时，migration 会自动执行，并补齐最小系统 bootstrap 数据"
echo ""
print_success "祝开发愉快！🚀"
echo ""
