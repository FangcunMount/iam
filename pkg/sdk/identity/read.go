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

// GetProfile 获取单个档案。
func (c *Client) GetProfile(ctx context.Context, profileID string) (*identityv1.GetProfileResponse, error) {
	resp, err := c.readService.GetProfile(ctx, &identityv1.GetProfileRequest{
		ProfileId: profileID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchGetProfiles 批量获取档案。
func (c *Client) BatchGetProfiles(ctx context.Context, profileIDs []string) (*identityv1.BatchGetProfilesResponse, error) {
	resp, err := c.readService.BatchGetProfiles(ctx, &identityv1.BatchGetProfilesRequest{
		ProfileIds: profileIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
