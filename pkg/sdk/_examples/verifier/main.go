// Token 验证示例
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
	authjwks "github.com/FangcunMount/iam/v2/pkg/sdk/auth/jwks"
	authverifier "github.com/FangcunMount/iam/v2/pkg/sdk/auth/verifier"
)

func main() {
	ctx := context.Background()

	// 创建客户端
	client, err := sdk.NewClient(ctx, &sdk.Config{
		Endpoint: "iam.example.com:8081",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 方式1：创建 JWKS 管理器 + Token 验证器（推荐高频场景）
	jwksManager, err := authjwks.NewJWKSManager(
		&sdk.JWKSConfig{
			URL:             "https://iam.example.com/.well-known/jwks.json",
			RefreshInterval: 5 * time.Minute,
			RequestTimeout:  5 * time.Second,
			FallbackOnError: true,
		},
		authjwks.WithCacheEnabled(true),
		authjwks.WithAuthClient(client.Auth()),
	)
	if err != nil {
		log.Fatalf("创建 JWKS 管理器失败: %v", err)
	}
	defer jwksManager.Stop()

	verifier, err := authverifier.NewTokenVerifier(
		&sdk.TokenVerifyConfig{
			AllowedAudience: []string{"my-app"},
			AllowedIssuer:   "https://iam.example.com",
			ClockSkew:       5 * time.Minute,
		},
		jwksManager,
		client.Auth(), // 可选：用于远程验证降级
	)
	if err != nil {
		log.Fatalf("创建验证器失败: %v", err)
	}

	// 验证 Token
	token := "eyJhbGciOiJSUzI1NiIs..."
	result, err := verifier.Verify(ctx, token, nil)
	if err != nil {
		log.Printf("验证失败: %v", err)
		return
	}

	if result.Valid {
		fmt.Printf("Token 有效\n")
		fmt.Printf("用户 ID: %s\n", result.Claims.UserID)
		fmt.Printf("会话 ID: %s\n", result.Claims.SessionID)
		fmt.Printf("租户 ID: %s\n", result.Claims.TenantID)
		fmt.Printf("过期时间: %s\n", result.Claims.ExpiresAt)
	}
}
