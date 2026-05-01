package identity

import (
	"context"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// HasProfileLink 判断用户是否为档案关系用户。
func (c *ProfileLinkClient) HasProfileLink(ctx context.Context, userID, profileID string) (*identityv2.HasProfileLinkResponse, error) {
	resp, err := c.queryService.HasProfileLink(ctx, &identityv2.HasProfileLinkRequest{
		UserId:    userID,
		ProfileId: profileID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListProfiles 列出用户的关系档案。
func (c *ProfileLinkClient) ListProfiles(ctx context.Context, req *identityv2.ListProfilesRequest) (*identityv2.ListProfilesResponse, error) {
	resp, err := c.queryService.ListProfiles(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetUserProfiles 使用默认分页列出用户的关系档案。
func (c *ProfileLinkClient) GetUserProfiles(ctx context.Context, userID string) (*identityv2.ListProfilesResponse, error) {
	resp, err := c.queryService.ListProfiles(ctx, &identityv2.ListProfilesRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListProfileLinks 列出档案的关系用户。
func (c *ProfileLinkClient) ListProfileLinks(ctx context.Context, req *identityv2.ListProfileLinksRequest) (*identityv2.ListProfileLinksResponse, error) {
	resp, err := c.queryService.ListProfileLinks(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
