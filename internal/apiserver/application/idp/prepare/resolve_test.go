package prepare

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

type fakeAppRepo struct{ app *idpPort.WechatApp }

func (f *fakeAppRepo) Create(context.Context, *idpPort.WechatApp) error { return nil }
func (f *fakeAppRepo) GetByID(context.Context, idutil.ID) (*idpPort.WechatApp, error) {
	return f.app, nil
}
func (f *fakeAppRepo) GetByAppID(context.Context, string) (*idpPort.WechatApp, error) {
	return f.app, nil
}
func (f *fakeAppRepo) List(context.Context, idpPort.ListFilter) ([]*idpPort.WechatApp, error) {
	return nil, nil
}
func (f *fakeAppRepo) Update(context.Context, *idpPort.WechatApp) error { return nil }

type fakeVault struct{}

func (fakeVault) Encrypt(_ context.Context, p []byte) ([]byte, error) { return p, nil }
func (fakeVault) Decrypt(_ context.Context, c []byte) ([]byte, error) { return c, nil }
func (fakeVault) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func appWithType(t idpPort.AppType) *idpPort.WechatApp {
	return &idpPort.WechatApp{
		AppID:  "wx-app",
		Type:   t,
		Status: idpPort.StatusEnabled,
		Cred: &idpPort.Credentials{
			Auth: &idpPort.AuthSecret{AppSecretCipher: []byte("secret")},
		},
	}
}

func TestResolveAppSecretRejectsTypeMismatch(t *testing.T) {
	deps := Dependencies{Apps: &fakeAppRepo{app: appWithType(idpPort.MP)}, Vault: fakeVault{}}
	_, err := ResolveAppSecret(context.Background(), deps, Options{
		Provider:        ProviderWechat,
		Surface:         SurfaceLoginProof,
		AppID:           "wx-app",
		ExpectedAppType: idpPort.OpenPlatformWebsite,
	})
	require.Error(t, err)
	require.Equal(t, code.ErrWechatAppTypeMismatch, perrors.ParseCoder(err).Code())
}

func TestResolveAppSecretAcceptsMatchingType(t *testing.T) {
	deps := Dependencies{Apps: &fakeAppRepo{app: appWithType(idpPort.OpenPlatformWebsite)}, Vault: fakeVault{}}
	secret, err := ResolveAppSecret(context.Background(), deps, Options{
		Provider:        ProviderWechat,
		Surface:         SurfaceLinking,
		AppID:           "wx-app",
		ExpectedAppType: idpPort.OpenPlatformWebsite,
	})
	require.NoError(t, err)
	require.Equal(t, "secret", secret)
}

func TestResolveAppSecretSkipsTypeCheckWhenUnset(t *testing.T) {
	deps := Dependencies{Apps: &fakeAppRepo{app: appWithType(idpPort.MiniProgram)}, Vault: fakeVault{}}
	secret, err := ResolveAppSecret(context.Background(), deps, Options{
		Provider: ProviderWechat,
		Surface:  SurfaceLinking,
		AppID:    "wx-app",
	})
	require.NoError(t, err)
	require.Equal(t, "secret", secret)
}
