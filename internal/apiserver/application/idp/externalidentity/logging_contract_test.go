package externalidentity_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/stretchr/testify/require"
)

func TestLoginResolutionLogExcludesProviderSecrets(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "external-identity.log")
	options := log.NewOptions()
	options.Format = "json"
	options.OutputPaths = []string{logPath}
	options.ErrorOutputPaths = []string{logPath}
	log.Init(options)
	t.Cleanup(func() {
		log.Flush()
		log.Init(log.NewOptions())
	})

	const (
		codeSentinel        = "provider-code-sentinel"
		secretSentinel      = "provider-secret-sentinel"
		tokenSentinel       = "provider-token-sentinel"
		rawResponseSentinel = "provider-raw-response-sentinel"
	)
	resolver := idpresolver.NewResolver(idpresolver.Dependencies{
		Apps: loggingAppRepository{app: &wechatapp.WechatApp{
			AppID:  "mini-app",
			Type:   wechatapp.MiniProgram,
			Status: wechatapp.StatusEnabled,
			Cred: &wechatapp.Credentials{Auth: &wechatapp.AuthSecret{
				AppSecretCipher: []byte("cipher"),
			}},
		}},
		Vault: loggingSecretVault{plain: []byte(secretSentinel)},
		Exchanger: loggingProviderExchanger{err: errors.New(strings.Join([]string{
			rawResponseSentinel,
			tokenSentinel,
			secretSentinel,
		}, " "))},
	})

	_, resolveErr := resolver.Resolve(context.Background(), idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    "mini-app",
		Code:     codeSentinel,
	})
	require.Error(t, resolveErr)
	require.Error(t, authnexternal.MapLoginProofError(context.Background(), resolveErr, "wechat_minip"))
	log.Flush()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	output := string(content)
	require.Contains(t, output, string(idpresolver.ErrorProviderExchange))
	require.Contains(t, output, string(idpidentity.ProviderWechatMinip))
	for _, forbidden := range []string{codeSentinel, secretSentinel, tokenSentinel, rawResponseSentinel} {
		require.NotContains(t, output, forbidden)
	}
}

type loggingAppRepository struct {
	app *wechatapp.WechatApp
}

func (s loggingAppRepository) Create(context.Context, *wechatapp.WechatApp) error { return nil }
func (s loggingAppRepository) GetByID(context.Context, idutil.ID) (*wechatapp.WechatApp, error) {
	return s.app, nil
}
func (s loggingAppRepository) GetByAppID(context.Context, string) (*wechatapp.WechatApp, error) {
	return s.app, nil
}
func (s loggingAppRepository) List(context.Context, wechatapp.ListFilter) ([]*wechatapp.WechatApp, error) {
	return nil, nil
}
func (s loggingAppRepository) Update(context.Context, *wechatapp.WechatApp) error { return nil }

type loggingSecretVault struct {
	plain []byte
}

func (s loggingSecretVault) Encrypt(context.Context, []byte) ([]byte, error) { return nil, nil }
func (s loggingSecretVault) Decrypt(context.Context, []byte) ([]byte, error) { return s.plain, nil }
func (s loggingSecretVault) Sign(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

type loggingProviderExchanger struct {
	err error
}

func (s loggingProviderExchanger) ExchangeWxMinipCode(context.Context, string, string, string) (string, string, error) {
	return "", "", s.err
}
func (s loggingProviderExchanger) ExchangeWxOpenCode(context.Context, string, string, string) (string, string, error) {
	return "", "", s.err
}
func (s loggingProviderExchanger) ExchangeWecomCode(context.Context, string, string, string, string) (string, string, error) {
	return "", "", s.err
}
