package authz

import (
	"context"

	authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// Check 对单条 (subject, domain, object, action) 执行授权判定。
func (c *Client) Check(ctx context.Context, req *authzv1.CheckRequest) (*authzv1.CheckResponse, error) {
	resp, err := c.authorizationService.Check(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Allow 是 Check 的便捷封装，只返回最终布尔结果。
func (c *Client) Allow(ctx context.Context, subject, domain, object, action string) (bool, error) {
	resp, err := c.Check(ctx, &authzv1.CheckRequest{
		Subject: subject,
		Domain:  domain,
		Object:  object,
		Action:  action,
	})
	if err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

// GetAuthorizationSnapshot 获取主体在指定租户/应用下的授权快照。
func (c *Client) GetAuthorizationSnapshot(ctx context.Context, req *authzv1.GetAuthorizationSnapshotRequest) (*authzv1.GetAuthorizationSnapshotResponse, error) {
	resp, err := c.authorizationService.GetAuthorizationSnapshot(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
