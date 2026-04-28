#!/bin/bash

# Proto文件生成脚本
# 用于生成gRPC相关的Go代码

set -e

echo "🔧 生成Proto文件..."

# 设置根目录
ROOT_DIR=$(cd "$(dirname "$0")/../.." && pwd)
cd "$ROOT_DIR"

# Proto 源文件路径
PROTO_PATH="api/grpc"

# Contract rule:
# Only generate proto contracts for implemented and registered runtime services.
# The current IAM gRPC public surface is v1-only. A future v2 proto must be added
# together with runtime registration, generated code, and SDK compile tests.

# 检查protoc是否安装
if ! command -v protoc &> /dev/null; then
    echo "❌ protoc 未安装，请先安装 Protocol Buffers"
    exit 1
fi

# 检查Go插件是否安装
if ! command -v protoc-gen-go &> /dev/null; then
    echo "❌ protoc-gen-go 未安装，正在安装..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "❌ protoc-gen-go-grpc 未安装，正在安装..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# 查找所有 proto 文件
PROTO_FILES=$(find ${PROTO_PATH} -name "*.proto")

if [ -z "$PROTO_FILES" ]; then
    echo "⚠️  未找到 proto 文件"
    exit 0
fi

echo "📁 发现以下 proto 文件:"
echo "$PROTO_FILES"

# 生成 authn proto
echo "🔄 生成 authn 服务..."
protoc \
    --proto_path=${PROTO_PATH} \
    --go_out=${PROTO_PATH} \
    --go_opt=paths=source_relative \
    --go-grpc_out=${PROTO_PATH} \
    --go-grpc_opt=paths=source_relative \
    iam/authn/v1/authn.proto

# 生成 identity proto
echo "🔄 生成 identity 服务..."
protoc \
    --proto_path=${PROTO_PATH} \
    --go_out=${PROTO_PATH} \
    --go_opt=paths=source_relative \
    --go-grpc_out=${PROTO_PATH} \
    --go-grpc_opt=paths=source_relative \
    iam/identity/v1/identity.proto

# 生成 idp proto
echo "🔄 生成 idp 服务..."
protoc \
    --proto_path=${PROTO_PATH} \
    --go_out=${PROTO_PATH} \
    --go_opt=paths=source_relative \
    --go-grpc_out=${PROTO_PATH} \
    --go-grpc_opt=paths=source_relative \
    iam/idp/v1/idp.proto

# 生成 authz proto
echo "🔄 生成 authz 服务..."
protoc \
    --proto_path=${PROTO_PATH} \
    --go_out=${PROTO_PATH} \
    --go_opt=paths=source_relative \
    --go-grpc_out=${PROTO_PATH} \
    --go-grpc_opt=paths=source_relative \
    iam/authz/v1/authz.proto

echo "✅ Proto文件生成完成！"
echo ""
echo "📁 生成的文件:"
find ${PROTO_PATH} -name "*.pb.go" -o -name "*_grpc.pb.go" | sort
 
