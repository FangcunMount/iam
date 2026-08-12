// 服务间认证示例
package main

import (
	"context"
	"log"
	"time"

	sdk "github.com/FangcunMount/iam/v3/pkg/sdk"
	authserviceauth "github.com/FangcunMount/iam/v3/pkg/sdk/auth/serviceauth"
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

	// 创建服务间认证助手
	authHelper, err := authserviceauth.NewServiceAuthHelper(&sdk.ServiceAuthConfig{
		ServiceID:      "qs-service",            // 当前服务标识
		TargetAudience: []string{"iam-service"}, // 目标服务
		TokenTTL:       time.Hour,               // Token 有效期
		RefreshBefore:  5 * time.Minute,         // 提前刷新时间
	}, client.Auth())
	if err != nil {
		log.Fatalf("创建认证助手失败: %v", err)
	}
	defer authHelper.Stop()

	// 方式1：获取带认证的 Context
	authCtx, err := authHelper.NewAuthenticatedContext(ctx)
	if err != nil {
		log.Fatalf("获取认证上下文失败: %v", err)
	}

	// 使用带认证的 Context 调用其他服务
	resp, err := client.Identity().GetUser(authCtx, "user-123")
	if err != nil {
		log.Printf("调用失败: %v", err)
		return
	}
	log.Printf("用户: %s", resp.User.Nickname)

	// 方式2：使用回调函数
	err = authHelper.CallWithAuth(ctx, func(authCtx context.Context) error {
		_, err := client.Identity().GetUser(authCtx, "user-456")
		return err
	})
	if err != nil {
		log.Printf("调用失败: %v", err)
	}
}
