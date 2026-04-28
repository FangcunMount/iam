// 基础用法示例
package main

import (
	"context"
	"fmt"
	"log"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	sdk "github.com/FangcunMount/iam/pkg/sdk"
)

func main() {
	ctx := context.Background()

	// 方式1: 手动配置
	client, err := sdk.NewClient(ctx, &sdk.Config{
		Endpoint: "localhost:8081",
		// 开发环境：不启用 TLS
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 方式2: 从环境变量加载
	// cfg, _ := sdk.ConfigFromEnv()
	// client, _ := sdk.NewClient(ctx, cfg)

	// 使用身份服务
	resp, err := client.Identity().GetUser(ctx, "user-123")
	if err != nil {
		log.Printf("获取用户失败: %v", err)
		return
	}
	fmt.Printf("用户: %s\n", resp.User.Nickname)

	// 搜索用户
	searchResp, err := client.Identity().SearchUsers(ctx, &identityv1.SearchUsersRequest{
		Keyword: "张三",
		Page:    &identityv1.OffsetPagination{Limit: 10, Offset: 0},
	})
	if err != nil {
		log.Printf("搜索用户失败: %v", err)
		return
	}
	fmt.Printf("找到 %d 个用户\n", searchResp.Total)

	// 检查监护关系
	guardianResp, err := client.Guardianship().IsGuardian(ctx, "user-123", "child-456")
	if err != nil {
		log.Printf("检查监护关系失败: %v", err)
		return
	}
	if guardianResp.IsGuardian {
		fmt.Println("是监护人")
	}
}
