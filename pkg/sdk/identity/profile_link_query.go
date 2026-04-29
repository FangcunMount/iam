package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// HasProfileLink 判断用户是否为档案关系用户。
func (c *ProfileLinkClient) HasProfileLink(ctx context.Context, userID, profileID string) (*identityv1.HasProfileLinkResponse, error) {
	resp, err := c.queryService.HasProfileLink(ctx, &identityv1.HasProfileLinkRequest{
		UserId:    userID,
		ProfileId: profileID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListProfiles 列出用户的关系档案。
func (c *ProfileLinkClient) ListProfiles(ctx context.Context, req *identityv1.ListProfilesRequest) (*identityv1.ListProfilesResponse, error) {
	resp, err := c.queryService.ListProfiles(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetUserProfiles 使用默认分页列出用户的关系档案。
func (c *ProfileLinkClient) GetUserProfiles(ctx context.Context, userID string) (*identityv1.ListProfilesResponse, error) {
	resp, err := c.queryService.ListProfiles(ctx, &identityv1.ListProfilesRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListProfileLinks 列出档案的关系用户。
func (c *ProfileLinkClient) ListProfileLinks(ctx context.Context, req *identityv1.ListProfileLinksRequest) (*identityv1.ListProfileLinksResponse, error) {
	resp, err := c.queryService.ListProfileLinks(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
