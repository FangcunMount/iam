package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// CreateUser 创建用户。
func (c *Client) CreateUser(ctx context.Context, req *identityv1.CreateUserRequest) (*identityv1.CreateUserResponse, error) {
	resp, err := c.lifecycleService.CreateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// UpdateUser 更新用户。
func (c *Client) UpdateUser(ctx context.Context, req *identityv1.UpdateUserRequest) (*identityv1.UpdateUserResponse, error) {
	resp, err := c.lifecycleService.UpdateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// DeactivateUser 停用用户。
func (c *Client) DeactivateUser(ctx context.Context, req *identityv1.ChangeUserStatusRequest) (*identityv1.UserOperationResponse, error) {
	resp, err := c.lifecycleService.DeactivateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BlockUser 封禁用户。
func (c *Client) BlockUser(ctx context.Context, req *identityv1.ChangeUserStatusRequest) (*identityv1.UserOperationResponse, error) {
	resp, err := c.lifecycleService.BlockUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
