#!/bin/bash

# 测试 Air 配置和证书设置

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧪 测试开发环境配置${NC}"
echo "========================================"

# 1. 检查证书文件
echo -e "\n${YELLOW}[1/4]${NC} 检查证书文件..."
if [ -f "configs/cert/web-apiserver.crt" ] && [ -f "configs/cert/web-apiserver.key" ]; then
    echo -e "${GREEN}✓ 证书文件存在${NC}"
else
    echo -e "${RED}✗ 证书文件不存在，正在生成...${NC}"
    ./scripts/cert/generate-dev-cert.sh
fi

# 2. 检查开发配置文件
echo -e "\n${YELLOW}[2/4]${NC} 检查开发配置文件..."
if [ -f "configs/apiserver.dev.yaml" ]; then
    echo -e "${GREEN}✓ 开发配置文件存在${NC}"
    
    # 验证证书路径
    CERT_PATH=$(grep "cert:" configs/apiserver.dev.yaml | head -1 | awk '{print $2}')
    KEY_PATH=$(grep "key:" configs/apiserver.dev.yaml | head -1 | awk '{print $2}')
    
    echo -e "${BLUE}   证书路径: $CERT_PATH${NC}"
    echo -e "${BLUE}   私钥路径: $KEY_PATH${NC}"
    
    if [[ "$CERT_PATH" == *"/etc/iam"* ]]; then
        echo -e "${RED}   ⚠️  警告: 配置文件使用的是生产环境路径${NC}"
        echo -e "${YELLOW}   建议修改为: configs/cert/web-apiserver.crt${NC}"
    else
        echo -e "${GREEN}   ✓ 证书路径正确（开发环境）${NC}"
    fi
else
    echo -e "${RED}✗ 开发配置文件不存在${NC}"
    exit 1
fi

# 3. 检查 Air 配置
echo -e "\n${YELLOW}[3/4]${NC} 检查 Air 配置..."
if [ -f ".air-apiserver.toml" ]; then
    echo -e "${GREEN}✓ Air 配置文件存在${NC}"
    
    # 检查使用的配置文件
    AIR_CONFIG=$(grep "args_bin\|full_bin" .air-apiserver.toml | grep "apiserver" | head -1)
    echo -e "${BLUE}   Air 配置: $AIR_CONFIG${NC}"
    
    if [[ "$AIR_CONFIG" == *"apiserver.dev.yaml"* ]]; then
        echo -e "${GREEN}   ✓ 使用开发环境配置${NC}"
    else
        echo -e "${RED}   ✗ 未使用开发环境配置${NC}"
    fi
else
    echo -e "${RED}✗ Air 配置文件不存在${NC}"
    exit 1
fi

# 4. 测试编译
echo -e "\n${YELLOW}[4/4]${NC} 测试编译..."
if go build -o ./tmp/apiserver ./cmd/apiserver/apiserver.go 2>&1; then
    echo -e "${GREEN}✓ 编译成功${NC}"
    
    # 测试配置文件读取
    echo -e "\n${BLUE}测试配置文件读取...${NC}"
    ./tmp/apiserver -c configs/apiserver.dev.yaml 2>&1 | head -10 &
    PID=$!
    sleep 2
    kill $PID 2>/dev/null || true
    
else
    echo -e "${RED}✗ 编译失败${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 配置测试完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}📝 下一步:${NC}"
echo "   make dev          # 启动开发环境"
echo "   make docker-dev-up   # 启动 Docker 开发环境"
echo ""
