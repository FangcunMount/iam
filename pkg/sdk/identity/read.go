package identity

import (
	"context"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// GetUser 获取单个用户。
func (c *Client) GetUser(ctx context.Context, userID string) (*identityv2.GetUserResponse, error) {
	resp, err := c.readService.GetUser(ctx, &identityv2.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchGetUsers 批量获取用户。
func (c *Client) BatchGetUsers(ctx context.Context, userIDs []string) (*identityv2.BatchGetUsersResponse, error) {
	resp, err := c.readService.BatchGetUsers(ctx, &identityv2.BatchGetUsersRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// SearchUsers 搜索用户。
func (c *Client) SearchUsers(ctx context.Context, req *identityv2.SearchUsersRequest) (*identityv2.SearchUsersResponse, error) {
	resp, err := c.readService.SearchUsers(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetProfile 获取单个档案。
func (c *Client) GetProfile(ctx context.Context, profileID string) (*identityv2.GetProfileResponse, error) {
	resp, err := c.readService.GetProfile(ctx, &identityv2.GetProfileRequest{
		ProfileId: profileID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchGetProfiles 批量获取档案。
func (c *Client) BatchGetProfiles(ctx context.Context, profileIDs []string) (*identityv2.BatchGetProfilesResponse, error) {
	resp, err := c.readService.BatchGetProfiles(ctx, &identityv2.BatchGetProfilesRequest{
		ProfileIds: profileIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
