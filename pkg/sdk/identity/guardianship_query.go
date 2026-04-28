package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// IsGuardian 判断用户是否为儿童监护人。
func (c *GuardianshipClient) IsGuardian(ctx context.Context, userID, childID string) (*identityv1.IsGuardianResponse, error) {
	resp, err := c.queryService.IsGuardian(ctx, &identityv1.IsGuardianRequest{
		UserId:  userID,
		ChildId: childID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListChildren 列出用户的监护儿童。
func (c *GuardianshipClient) ListChildren(ctx context.Context, req *identityv1.ListChildrenRequest) (*identityv1.ListChildrenResponse, error) {
	resp, err := c.queryService.ListChildren(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// GetUserChildren 使用默认分页列出用户的监护儿童。
func (c *GuardianshipClient) GetUserChildren(ctx context.Context, userID string) (*identityv1.ListChildrenResponse, error) {
	resp, err := c.queryService.ListChildren(ctx, &identityv1.ListChildrenRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ListGuardians 列出儿童的监护人。
func (c *GuardianshipClient) ListGuardians(ctx context.Context, req *identityv1.ListGuardiansRequest) (*identityv1.ListGuardiansResponse, error) {
	resp, err := c.queryService.ListGuardians(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
