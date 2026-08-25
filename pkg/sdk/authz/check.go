package authz

import (
	"context"

	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	"github.com/FangcunMount/iam/v3/pkg/sdk/errors"
)

func (c *Client) Check(ctx context.Context, req *authzv3.CheckRequest) (*authzv3.CheckResponse, error) {
	resp, err := c.authorizationService.Check(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// Allow performs an unconditional resource/action check. Conditional grants
// fail closed because no ObjectContext attributes are supplied.
func (c *Client) Allow(ctx context.Context, subject, domain, resource, action string) (bool, error) {
	resp, err := c.Check(ctx, &authzv3.CheckRequest{
		Subject: subject, Domain: domain, Resource: resource, Action: action,
	})
	if err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

func (c *Client) CheckObject(
	ctx context.Context,
	subject, domain, resource, action, objectID string,
	attributes []*authzv3.ObjectAttribute,
) (*authzv3.CheckResponse, error) {
	return c.Check(ctx, &authzv3.CheckRequest{
		Subject: subject, Domain: domain, Resource: resource, Action: action,
		ObjectContext: &authzv3.ObjectContext{ObjectId: objectID, Attributes: attributes},
	})
}

func (c *Client) GetAuthorizationSnapshot(ctx context.Context, req *authzv3.GetAuthorizationSnapshotRequest) (*authzv3.GetAuthorizationSnapshotResponse, error) {
	resp, err := c.authorizationService.GetAuthorizationSnapshot(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
