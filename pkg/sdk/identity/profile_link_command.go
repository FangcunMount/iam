package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// EstablishProfileLink 创建档案关系。
func (c *ProfileLinkClient) EstablishProfileLink(ctx context.Context, req *identityv1.EstablishProfileLinkRequest) (*identityv1.EstablishProfileLinkResponse, error) {
	resp, err := c.commandService.EstablishProfileLink(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeProfileLink 撤销档案关系。
func (c *ProfileLinkClient) RevokeProfileLink(ctx context.Context, req *identityv1.RevokeProfileLinkRequest) (*identityv1.RevokeProfileLinkResponse, error) {
	resp, err := c.commandService.RevokeProfileLink(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchRevokeProfileLinks 批量撤销档案关系。
func (c *ProfileLinkClient) BatchRevokeProfileLinks(ctx context.Context, req *identityv1.BatchRevokeProfileLinksRequest) (*identityv1.BatchRevokeProfileLinksResponse, error) {
	resp, err := c.commandService.BatchRevokeProfileLinks(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ImportProfileLinks 批量导入档案关系。
func (c *ProfileLinkClient) ImportProfileLinks(ctx context.Context, req *identityv1.ImportProfileLinksRequest) (*identityv1.ImportProfileLinksResponse, error) {
	resp, err := c.commandService.ImportProfileLinks(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
