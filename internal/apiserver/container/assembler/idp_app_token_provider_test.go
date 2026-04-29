package assembler

import (
	"context"
	"testing"

	wechatappDomain "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/stretchr/testify/require"
)

func TestAppTokenProviderAdapterCurrentBehaviorRequiresApplicationLayerCredentials(t *testing.T) {
	adapter := &appTokenProviderAdapter{}
	app := &wechatappDomain.WechatApp{
		AppID: "wx-app",
		Cred: &wechatappDomain.Credentials{
			Auth: &wechatappDomain.AuthSecret{
				AppSecretCipher: []byte("ciphertext"),
			},
		},
	}

	token, err := adapter.Fetch(context.Background(), app)

	require.Nil(t, token)
	require.ErrorContains(t, err, "not implemented")
	require.ErrorContains(t, err, "decrypted credentials")
}
