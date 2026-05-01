package identity

import (
	"context"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// CreateUser 创建用户。
func (c *Client) CreateUser(ctx context.Context, req *identityv2.CreateUserRequest) (*identityv2.CreateUserResponse, error) {
	resp, err := c.lifecycleService.CreateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// UpdateUser 更新用户。
func (c *Client) UpdateUser(ctx context.Context, req *identityv2.UpdateUserRequest) (*identityv2.UpdateUserResponse, error) {
	resp, err := c.lifecycleService.UpdateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// DeactivateUser 停用用户。
func (c *Client) DeactivateUser(ctx context.Context, req *identityv2.ChangeUserStatusRequest) (*identityv2.UserOperationResponse, error) {
	resp, err := c.lifecycleService.DeactivateUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BlockUser 封禁用户。
func (c *Client) BlockUser(ctx context.Context, req *identityv2.ChangeUserStatusRequest) (*identityv2.UserOperationResponse, error) {
	resp, err := c.lifecycleService.BlockUser(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
