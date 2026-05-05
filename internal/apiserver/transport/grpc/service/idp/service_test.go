package idp

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type wechatTokenServiceStub struct {
	getToken     string
	refreshToken string
	err          error
}

func (s wechatTokenServiceStub) GetAccessToken(context.Context, string) (string, error) {
	return s.getToken, s.err
}

func (s wechatTokenServiceStub) RefreshAccessToken(context.Context, string) (string, error) {
	return s.refreshToken, s.err
}

func TestIDPServerWechatAccessToken(t *testing.T) {
	srv := &idpServer{wechatAppTokenService: wechatTokenServiceStub{getToken: "cached-token", refreshToken: "fresh-token"}}

	got, err := srv.GetWechatAccessToken(context.Background(), &idpv2.GetWechatAccessTokenRequest{AppId: "wx-app"})
	require.NoError(t, err)
	require.Equal(t, "cached-token", got.GetAccessToken())

	refreshed, err := srv.RefreshWechatAccessToken(context.Background(), &idpv2.RefreshWechatAccessTokenRequest{AppId: "wx-app"})
	require.NoError(t, err)
	require.Equal(t, "fresh-token", refreshed.GetAccessToken())
}

func TestIDPServerWechatAccessTokenMapsStructuredNotFound(t *testing.T) {
	srv := &idpServer{
		wechatAppTokenService: wechatTokenServiceStub{
			err: perrors.WithCode(code.ErrWechatAppNotFound, "wechat app not found: missing"),
		},
	}

	_, err := srv.GetWechatAccessToken(context.Background(), &idpv2.GetWechatAccessTokenRequest{AppId: "missing"})

	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
