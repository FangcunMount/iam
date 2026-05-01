package idp

import (
	"context"

	idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/errors"
)

// GetWechatApp 根据 AppID 查询微信应用。
func (c *Client) GetWechatApp(ctx context.Context, appID string) (*idpv2.GetWechatAppResponse, error) {
	resp, err := c.idpService.GetWechatApp(ctx, &idpv2.GetWechatAppRequest{
		AppId: appID,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return resp, nil
}
