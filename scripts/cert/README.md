# 证书管理说明

## 📢 重要变更

IAM 项目不再维护证书生成脚本，也**不再复制证书**。所有证书由 **infra 项目** 统一管理，IAM 项目配置文件**直接引用 infra 路径**。

## 🏗️ 架构说明

```text
/data/infra/ssl/                ← 统一证书根目录
├── grpc/                       ← gRPC mTLS 证书
│   ├── ca/
│   │   ├── root-ca.crt
│   │   ├── intermediate-ca.crt
│   │   └── ca-chain.crt        ← 所有项目引用
│   └── server/
│       ├── iam-apiserver.crt   ← IAM gRPC 配置引用
│       ├── iam-apiserver.key
│       ├── qs-apiserver.crt
│       ├── qs-collection-server.crt
│       └── qs-worker.crt
└── web/                        ← REST API HTTPS 证书
    ├── iam-apiserver.crt       ← IAM HTTPS 配置引用
    ├── iam-apiserver.key
    └── ...
```

## 🚀 快速开始

### 1. 在 infra 项目生成 CA 证书（首次运行）

```bash
cd /path/to/infra
./scripts/cert/generate-grpc-certs.sh generate-ca
```

### 2. 在 infra 项目为 IAM 生成服务端证书

```bash
cd /path/to/infra
./scripts/04-ssl/grpc-mtls.sh generate-server iam-apiserver IAM iam-apiserver.internal.example.com
```

### 3. IAM 配置直接引用 infra 路径

```yaml
# configs/apiserver.yaml

# REST API HTTPS 配置
tls:
  cert: /data/infra/ssl/web/iam-apiserver.crt
  key: /data/infra/ssl/web/iam-apiserver.key

# gRPC mTLS 配置
grpc:
  mtls:
    cert-file: /data/infra/ssl/grpc/server/iam-apiserver.crt
    key-file: /data/infra/ssl/grpc/server/iam-apiserver.key
    ca-file: /data/infra/ssl/grpc/ca/ca-chain.crt
```

## 📁 路径约定

### gRPC mTLS 证书

| 证书类型 | 统一路径 | 说明 |
| --------- | --------- | ------ |
| CA 证书链 | `/data/infra/ssl/grpc/ca/ca-chain.crt` | 所有项目验证证书时引用 |
| IAM 服务端证书 | `/data/infra/ssl/grpc/server/iam-apiserver.crt` | IAM gRPC 配置引用 |
| IAM 服务端私钥 | `/data/infra/ssl/grpc/server/iam-apiserver.key` | IAM gRPC 配置引用 |
| QS apiserver 证书 | `/data/infra/ssl/grpc/server/qs-apiserver.crt` | QS gRPC 服务端证书 |
| QS collection 证书 | `/data/infra/ssl/grpc/server/qs-collection-server.crt` | QS gRPC 客户端证书 |
| QS worker 证书 | `/data/infra/ssl/grpc/server/qs-worker.crt` | QS gRPC 客户端证书 |

### REST API HTTPS 证书

| 证书类型 | 统一路径 | 说明 |
| --------- | --------- | ------ |
| IAM HTTPS 证书 | `/data/infra/ssl/web/iam-apiserver.crt` | IAM HTTPS 配置引用 |
| IAM HTTPS 私钥 | `/data/infra/ssl/web/iam-apiserver.key` | IAM HTTPS 配置引用 |

## ✅ 优势

1. **零复制**：配置直接引用，避免证书同步问题
2. **集中管理**：所有证书在一个目录，便于管理和审计
3. **一致性**：所有服务使用同一个 CA，证书链验证更简单
4. **安全性**：CA 私钥只存在于 infra 项目，降低泄漏风险
5. **简化维护**：各项目不需要维护证书生成和复制脚本

## 🔧 验证命令

```bash
# 验证证书
make grpc-cert-verify

# 查看证书信息
make grpc-cert-info
```

## 📖 详细文档

查看 [docs/00-概览/03-grpc服务设计.md](../../docs/00-概览/03-grpc服务设计.md) 了解完整的证书管理架构。
