package idp

import (
	"context"

	idpv1 "github.com/FangcunMount/iam/api/grpc/iam/idp/v1"
	"github.com/FangcunMount/iam/pkg/sdk/errors"
)

// GetWechatApp 根据 AppID 查询微信应用。
func (c *Client) GetWechatApp(ctx context.Context, appID string) (*idpv1.GetWechatAppResponse, error) {
	resp, err := c.idpService.GetWechatApp(ctx, &idpv1.GetWechatAppRequest{
		AppId: appID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
