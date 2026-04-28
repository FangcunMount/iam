// mTLS 生产环境配置示例
package main

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	sdk "github.com/FangcunMount/iam/pkg/sdk"
)

func main() {
	ctx := context.Background()

	// 生产环境配置：mTLS + 重试 + Keepalive
	cfg := &sdk.Config{
		Endpoint: "iam.example.com:8081",
		Timeout:  30 * time.Second,

		TLS: &sdk.TLSConfig{
			Enabled:    true,
			CACert:     "/etc/iam/certs/ca.crt",
			ClientCert: "/etc/iam/certs/client.crt",
			ClientKey:  "/etc/iam/certs/client.key",
			ServerName: "iam.example.com",
			MinVersion: tls.VersionTLS12,
		},

		Retry: &sdk.RetryConfig{
			Enabled:           true,
			MaxAttempts:       3,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        10 * time.Second,
			BackoffMultiplier: 2.0,
		},

		Keepalive: &sdk.KeepaliveConfig{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		},
	}

	client, err := sdk.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	// 使用客户端...
	log.Println("mTLS 客户端创建成功")
}
