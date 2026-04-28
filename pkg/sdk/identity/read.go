package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// GetUser 获取单个用户。
func (c *Client) GetUser(ctx context.Context, userID string) (*identityv1.GetUserResponse, error) {
	resp, err := c.readService.GetUser(ctx, &identityv1.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchGetUsers 批量获取用户。
func (c *Client) BatchGetUsers(ctx context.Context, userIDs []string) (*identityv1.BatchGetUsersResponse, error) {
	resp, err := c.readService.BatchGetUsers(ctx, &identityv1.BatchGetUsersRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// SearchUsers 搜索用户。
func (c *Client) SearchUsers(ctx context.Context, req *identityv1.SearchUsersRequest) (*identityv1.SearchUsersResponse, error) {
	resp, err := c.readService.SearchUsers(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetChild 获取单个儿童。
func (c *Client) GetChild(ctx context.Context, childID string) (*identityv1.GetChildResponse, error) {
	resp, err := c.readService.GetChild(ctx, &identityv1.GetChildRequest{
		ChildId: childID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchGetChildren 批量获取儿童。
func (c *Client) BatchGetChildren(ctx context.Context, childIDs []string) (*identityv1.BatchGetChildrenResponse, error) {
	resp, err := c.readService.BatchGetChildren(ctx, &identityv1.BatchGetChildrenRequest{
		ChildIds: childIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
