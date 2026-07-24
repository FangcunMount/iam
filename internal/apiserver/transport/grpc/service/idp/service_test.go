package idp

import (
	"context"
	"errors"
	"strings"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"
	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
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

type wechatAppRepositoryStub struct {
	app *wechatappDomain.WechatApp
	err error
}

func (s wechatAppRepositoryStub) Create(context.Context, *wechatappDomain.WechatApp) error {
	return s.err
}
func (s wechatAppRepositoryStub) GetByID(context.Context, idutil.ID) (*wechatappDomain.WechatApp, error) {
	return s.app, s.err
}
func (s wechatAppRepositoryStub) GetByAppID(context.Context, string) (*wechatappDomain.WechatApp, error) {
	return s.app, s.err
}
func (s wechatAppRepositoryStub) List(context.Context, wechatappDomain.ListFilter) ([]*wechatappDomain.WechatApp, error) {
	return []*wechatappDomain.WechatApp{s.app}, s.err
}
func (s wechatAppRepositoryStub) Update(context.Context, *wechatappDomain.WechatApp) error {
	return s.err
}

type failingSecretVault struct{ err error }

func (s failingSecretVault) Encrypt(context.Context, []byte) ([]byte, error) { return nil, s.err }
func (s failingSecretVault) Decrypt(context.Context, []byte) ([]byte, error) { return nil, s.err }
func (s failingSecretVault) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, s.err
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

func TestIDPServerDecryptFailureHidesInternalError(t *testing.T) {
	const sentinel = "idp-decrypt-internal-sentinel"
	app := wechatappDomain.NewWechatApp(
		wechatappDomain.MiniProgram,
		"wx-app",
		wechatappDomain.WithWechatAppStatus(wechatappDomain.StatusEnabled),
	)
	app.Cred = &wechatappDomain.Credentials{
		Auth: &wechatappDomain.AuthSecret{AppSecretCipher: []byte("ciphertext")},
	}
	srv := &idpServer{
		wechatAppRepo: wechatAppRepositoryStub{app: app},
		secretVault:   failingSecretVault{err: errors.New(sentinel)},
	}

	_, err := srv.GetWechatApp(context.Background(), &idpv2.GetWechatAppRequest{AppId: "wx-app"})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "internal server error", status.Convert(err).Message())
	require.False(t, strings.Contains(err.Error(), sentinel))
}
