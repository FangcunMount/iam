// 授权判定（PDP）示例
package main

import (
	"context"
	"log"

	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	sdk "github.com/FangcunMount/iam/v3/pkg/sdk"
)

func main() {
	ctx := context.Background()

	client, err := sdk.NewClient(ctx, &sdk.Config{
		Endpoint: "localhost:8081",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 方式 1：使用原始 Check 请求
	resp, err := client.Authz().Check(ctx, &authzv3.CheckRequest{
		Subject:  "user:123",
		Domain:   "fangcun",
		Resource: "iam:identity:instance:profile",
		Action:   "read",
	})
	if err != nil {
		log.Fatalf("Check 失败: %v", err)
	}
	log.Printf("Check allowed=%v", resp.Allowed)

	// 方式 2：使用便捷 Allow 封装
	allowed, err := client.Authz().Allow(
		ctx,
		"user:123",
		"fangcun",
		"qs:evaluation:collection:reports",
		"read",
	)
	if err != nil {
		log.Fatalf("Allow 失败: %v", err)
	}
	log.Printf("Allow result=%v", allowed)
}
