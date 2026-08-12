package identity

import (
	"context"

	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	"github.com/FangcunMount/iam/v3/pkg/sdk/errors"
)

// CreateProfile 创建档案并建立 User -> Profile 关系。
func (c *ProfileClient) CreateProfile(ctx context.Context, req *identityv2.CreateProfileRequest) (*identityv2.CreateProfileResponse, error) {
	resp, err := c.commandService.CreateProfile(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
