package idp

import (
	"context"
	"errors"
	"testing"
	"time"

	wechatappDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi"
	"github.com/stretchr/testify/require"
)

func TestAppTokenProviderAdapterFetchesMiniProgramToken(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	provider := &fakeWechatAccessTokenProvider{
		miniResult: &wechatapi.AccessTokenResult{Token: "mini-token", ExpiresAt: expiresAt},
	}
	adapter := &appTokenProviderAdapter{
		tokenProvider: provider,
		secretVault:   fakeSecretVault{plain: []byte("mini-secret")},
	}
	app := &wechatappDomain.WechatApp{
		AppID: "wx-app",
		Type:  wechatappDomain.MiniProgram,
		Cred: &wechatappDomain.Credentials{
			Auth: &wechatappDomain.AuthSecret{
				AppSecretCipher: []byte("ciphertext"),
			},
		},
	}

	token, err := adapter.Fetch(context.Background(), app)

	require.NoError(t, err)
	require.Equal(t, "mini-token", token.Token)
	require.Equal(t, expiresAt, token.ExpiresAt)
	require.Equal(t, "wx-app", provider.miniAppID)
	require.Equal(t, "mini-secret", provider.miniSecret)
	require.Empty(t, provider.officialAppID)
}

func TestAppTokenProviderAdapterFetchesOfficialAccountToken(t *testing.T) {
	expiresAt := time.Now().Add(2 * time.Hour)
	provider := &fakeWechatAccessTokenProvider{
		officialResult: &wechatapi.AccessTokenResult{Token: "mp-token", ExpiresAt: expiresAt},
	}
	adapter := &appTokenProviderAdapter{
		tokenProvider: provider,
		secretVault:   fakeSecretVault{plain: []byte("mp-secret")},
	}
	app := &wechatappDomain.WechatApp{
		AppID: "mp-app",
		Type:  wechatappDomain.MP,
		Cred: &wechatappDomain.Credentials{
			Auth: &wechatappDomain.AuthSecret{
				AppSecretCipher: []byte("ciphertext"),
			},
		},
	}

	token, err := adapter.Fetch(context.Background(), app)

	require.NoError(t, err)
	require.Equal(t, "mp-token", token.Token)
	require.Equal(t, expiresAt, token.ExpiresAt)
	require.Equal(t, "mp-app", provider.officialAppID)
	require.Equal(t, "mp-secret", provider.officialSecret)
	require.Empty(t, provider.miniAppID)
}

func TestAppTokenProviderAdapterReturnsDecryptErrorBeforeCallingProvider(t *testing.T) {
	provider := &fakeWechatAccessTokenProvider{}
	adapter := &appTokenProviderAdapter{
		tokenProvider: provider,
		secretVault:   fakeSecretVault{err: errors.New("decrypt failed")},
	}
	app := &wechatappDomain.WechatApp{
		AppID: "wx-app",
		Type:  wechatappDomain.MiniProgram,
		Cred: &wechatappDomain.Credentials{
			Auth: &wechatappDomain.AuthSecret{
				AppSecretCipher: []byte("ciphertext"),
			},
		},
	}

	token, err := adapter.Fetch(context.Background(), app)

	require.Nil(t, token)
	require.ErrorContains(t, err, "failed to decrypt app secret")
	require.Empty(t, provider.miniAppID)
	require.Empty(t, provider.officialAppID)
}

func TestAppTokenProviderAdapterRejectsUnsupportedAppType(t *testing.T) {
	provider := &fakeWechatAccessTokenProvider{}
	adapter := &appTokenProviderAdapter{
		tokenProvider: provider,
		secretVault:   fakeSecretVault{plain: []byte("secret")},
	}
	app := &wechatappDomain.WechatApp{
		AppID: "wx-app",
		Type:  wechatappDomain.AppType("Unknown"),
		Cred: &wechatappDomain.Credentials{
			Auth: &wechatappDomain.AuthSecret{
				AppSecretCipher: []byte("ciphertext"),
			},
		},
	}

	token, err := adapter.Fetch(context.Background(), app)

	require.Nil(t, token)
	require.ErrorContains(t, err, "unsupported wechat app type")
	require.Empty(t, provider.miniAppID)
	require.Empty(t, provider.officialAppID)
}

type fakeWechatAccessTokenProvider struct {
	miniResult     *wechatapi.AccessTokenResult
	miniErr        error
	miniAppID      string
	miniSecret     string
	officialResult *wechatapi.AccessTokenResult
	officialErr    error
	officialAppID  string
	officialSecret string
}

func (p *fakeWechatAccessTokenProvider) FetchMiniProgramToken(ctx context.Context, appID, appSecret string) (*wechatapi.AccessTokenResult, error) {
	p.miniAppID = appID
	p.miniSecret = appSecret
	return p.miniResult, p.miniErr
}

func (p *fakeWechatAccessTokenProvider) FetchOfficialAccountToken(ctx context.Context, appID, appSecret string) (*wechatapi.AccessTokenResult, error) {
	p.officialAppID = appID
	p.officialSecret = appSecret
	return p.officialResult, p.officialErr
}

type fakeSecretVault struct {
	plain []byte
	err   error
}

func (v fakeSecretVault) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (v fakeSecretVault) Decrypt(ctx context.Context, cipher []byte) ([]byte, error) {
	if v.err != nil {
		return nil, v.err
	}
	return append([]byte(nil), v.plain...), nil
}

func (v fakeSecretVault) Sign(ctx context.Context, keyRef string, data []byte) ([]byte, error) {
	return data, nil
}
