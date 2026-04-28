package identity

import (
	"context"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// AddGuardian 创建监护关系。
func (c *GuardianshipClient) AddGuardian(ctx context.Context, req *identityv1.AddGuardianRequest) (*identityv1.AddGuardianResponse, error) {
	resp, err := c.commandService.AddGuardian(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// RevokeGuardian 撤销监护关系。
func (c *GuardianshipClient) RevokeGuardian(ctx context.Context, req *identityv1.RevokeGuardianRequest) (*identityv1.RevokeGuardianResponse, error) {
	resp, err := c.commandService.RevokeGuardian(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// BatchRevokeGuardians 批量撤销监护关系。
func (c *GuardianshipClient) BatchRevokeGuardians(ctx context.Context, req *identityv1.BatchRevokeGuardiansRequest) (*identityv1.BatchRevokeGuardiansResponse, error) {
	resp, err := c.commandService.BatchRevokeGuardians(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}

// ImportGuardians 批量导入监护关系。
func (c *GuardianshipClient) ImportGuardians(ctx context.Context, req *identityv1.ImportGuardiansRequest) (*identityv1.ImportGuardiansResponse, error) {
	resp, err := c.commandService.ImportGuardians(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
