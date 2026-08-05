package config

import "fmt"

// GenerateEnvExample 生成环境变量配置示例。
func GenerateEnvExample(prefix string) string {
	return fmt.Sprintf(`# IAM SDK 环境变量配置示例
# 基础配置
%s_ENDPOINT=iam.example.com:8081
%s_TIMEOUT=30s
%s_DIAL_TIMEOUT=10s

# TLS 配置
%s_TLS_ENABLED=true
%s_TLS_CA_CERT=/etc/iam/certs/ca.crt
%s_TLS_CLIENT_CERT=/etc/iam/certs/client.crt
%s_TLS_CLIENT_KEY=/etc/iam/certs/client.key
%s_TLS_SERVER_NAME=iam.example.com
%s_TLS_SKIP_VERIFY=false

# 重试配置
%s_RETRY_ENABLED=true
%s_RETRY_MAX_ATTEMPTS=3
%s_RETRY_INITIAL_BACKOFF=100ms
%s_RETRY_MAX_BACKOFF=10s

# gRPC keepalive 配置（与 IAM 服务端 enforcement 契约一致）
%s_KEEPALIVE_ENABLED=true
%s_KEEPALIVE_TIME=5m
%s_KEEPALIVE_TIMEOUT=20s
%s_KEEPALIVE_PERMIT_WITHOUT_STREAM=false

# JWKS 配置
%s_JWKS_URL=https://iam.example.com/.well-known/jwks.json
%s_JWKS_REFRESH_INTERVAL=5m
`, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)
}

// GenerateYAMLExample 生成 YAML 配置示例。
func GenerateYAMLExample() string {
	return `# IAM SDK YAML 配置示例
iam:
  endpoint: "iam.example.com:8081"
  timeout: 30s
  dial_timeout: 10s
  load_balancer: round_robin

  tls:
    enabled: true
    ca_cert: "/etc/iam/certs/ca.crt"
    client_cert: "/etc/iam/certs/client.crt"
    client_key: "/etc/iam/certs/client.key"
    server_name: "iam.example.com"
    insecure_skip_verify: false

  retry:
    enabled: true
    max_attempts: 3
    initial_backoff: 100ms
    max_backoff: 10s

  keepalive:
    time: 5m
    timeout: 20s
    permit_without_stream: false

  jwks:
    url: "https://iam.example.com/.well-known/jwks.json"
    refresh_interval: 5m
    request_timeout: 5s
`
}
