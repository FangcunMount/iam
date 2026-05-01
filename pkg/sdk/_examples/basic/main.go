// 基础用法示例
package main

import (
	"context"
	"fmt"
	"log"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	sdk "github.com/FangcunMount/iam/v2/pkg/sdk"
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
	searchResp, err := client.Identity().SearchUsers(ctx, &identityv2.SearchUsersRequest{
		Keyword: "张三",
		Page:    &identityv2.OffsetPagination{Limit: 10, Offset: 0},
	})
	if err != nil {
		log.Printf("搜索用户失败: %v", err)
		return
	}
	fmt.Printf("找到 %d 个用户\n", searchResp.Total)

	// 检查档案关系
	linkResp, err := client.ProfileLink().HasProfileLink(ctx, "user-123", "profile-456")
	if err != nil {
		log.Printf("检查档案关系失败: %v", err)
		return
	}
	if linkResp.GetHasProfileLink() {
		fmt.Println("是关系用户")
	}
}
