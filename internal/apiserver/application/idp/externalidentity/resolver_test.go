package externalidentity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/util/idutil"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	wechatapp "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/stretchr/testify/require"
)

func TestResolverResolvesSupportedProviders(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	exchanger := &exchangerStub{}
	resolver := NewResolver(Dependencies{
		Apps:      appRepositoryStub{app: enabledApp(wechatapp.MiniProgram)},
		Vault:     vaultStub{plain: []byte("secret")},
		Exchanger: exchanger,
		Config:    Config{WeComAgentID: "agent-1"},
		Now:       func() time.Time { return verifiedAt },
	})

	identity, err := resolver.Resolve(context.Background(), ResolveRequest{
		Provider: domain.ProviderWechatMinip,
		Realm:    " app-1 ",
		Code:     " code-1 ",
	})
	require.NoError(t, err)
	require.Equal(t, "app-1", identity.Realm())
	require.Equal(t, verifiedAt, identity.VerifiedAt())
	require.Equal(t, 1, exchanger.miniCalls)
	openID, ok := identity.Identifier(domain.IdentifierOpenID)
	require.True(t, ok)
	require.Equal(t, "open-1", openID)
}

func TestResolverResolvesWechatOpen(t *testing.T) {
	exchanger := &exchangerStub{}
	resolver := NewResolver(Dependencies{
		Apps:      appRepositoryStub{app: enabledApp(wechatapp.OpenPlatformWebsite)},
		Vault:     vaultStub{plain: []byte("secret")},
		Exchanger: exchanger,
	})

	identity, err := resolver.Resolve(context.Background(), ResolveRequest{
		Provider: domain.ProviderWechatOpen,
		Realm:    "open-app",
		Code:     "oauth-code",
	})

	require.NoError(t, err)
	require.Equal(t, 1, exchanger.openCalls)
	openID, ok := identity.Identifier(domain.IdentifierOpenID)
	require.True(t, ok)
	require.Equal(t, "open-1", openID)
}

func TestResolverRejectsEmptyIdentifiersForEveryProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider domain.Provider
		appType  wechatapp.AppType
		config   Config
	}{
		{name: "wechat mini", provider: domain.ProviderWechatMinip, appType: wechatapp.MiniProgram},
		{name: "wechat open", provider: domain.ProviderWechatOpen, appType: wechatapp.OpenPlatformWebsite},
		{name: "wecom", provider: domain.ProviderWecom, appType: wechatapp.MP, config: Config{WeComAgentID: "agent-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(Dependencies{
				Apps:      appRepositoryStub{app: enabledApp(tt.appType)},
				Vault:     vaultStub{plain: []byte("secret")},
				Exchanger: &exchangerStub{empty: true},
				Config:    tt.config,
			})

			_, err := resolver.Resolve(context.Background(), ResolveRequest{
				Provider: tt.provider,
				Realm:    "realm-1",
				Code:     "code-1",
			})
			requireErrorKind(t, err, ErrorInvalidProviderReply)
		})
	}
}

func TestResolverClassifiesProviderRejectionAndTimeoutForEveryProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider domain.Provider
		appType  wechatapp.AppType
		config   Config
	}{
		{name: "wechat mini", provider: domain.ProviderWechatMinip, appType: wechatapp.MiniProgram},
		{name: "wechat open", provider: domain.ProviderWechatOpen, appType: wechatapp.OpenPlatformWebsite},
		{name: "wecom", provider: domain.ProviderWecom, appType: wechatapp.MP, config: Config{WeComAgentID: "agent-1"}},
	}
	causes := []struct {
		name string
		err  error
	}{
		{name: "provider rejected", err: errors.New("provider rejected code")},
		{name: "timeout", err: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		for _, cause := range causes {
			t.Run(tt.name+"/"+cause.name, func(t *testing.T) {
				resolver := NewResolver(Dependencies{
					Apps:      appRepositoryStub{app: enabledApp(tt.appType)},
					Vault:     vaultStub{plain: []byte("secret")},
					Exchanger: &exchangerStub{err: cause.err},
					Config:    tt.config,
				})

				_, err := resolver.Resolve(context.Background(), ResolveRequest{
					Provider: tt.provider,
					Realm:    "realm-1",
					Code:     "code-1",
				})
				requireErrorKind(t, err, ErrorProviderExchange)
				require.ErrorIs(t, err, cause.err)
			})
		}
	}
}

func TestResolverRejectsAppTypeMismatch(t *testing.T) {
	resolver := NewResolver(Dependencies{
		Apps:      appRepositoryStub{app: enabledApp(wechatapp.MP)},
		Vault:     vaultStub{plain: []byte("secret")},
		Exchanger: &exchangerStub{},
	})

	_, err := resolver.Resolve(context.Background(), ResolveRequest{
		Provider: domain.ProviderWechatOpen,
		Realm:    "app-1",
		Code:     "code-1",
	})
	requireErrorKind(t, err, ErrorAppTypeMismatch)
	resolutionError, ok := AsResolutionError(err)
	require.True(t, ok)
	require.Equal(t, wechatapp.OpenPlatformWebsite, resolutionError.Expected)
	require.Equal(t, wechatapp.MP, resolutionError.Actual)
}

func TestResolverClassifiesFailures(t *testing.T) {
	tests := []struct {
		name string
		deps Dependencies
		want ErrorKind
	}{
		{name: "unavailable", deps: Dependencies{}, want: ErrorUnavailable},
		{name: "query", deps: Dependencies{Apps: appRepositoryStub{err: errors.New("db")}, Vault: vaultStub{}, Exchanger: &exchangerStub{}}, want: ErrorAppQueryFailed},
		{name: "not found", deps: Dependencies{Apps: appRepositoryStub{}, Vault: vaultStub{}, Exchanger: &exchangerStub{}}, want: ErrorAppNotFound},
		{name: "disabled", deps: Dependencies{Apps: appRepositoryStub{app: disabledApp()}, Vault: vaultStub{}, Exchanger: &exchangerStub{}}, want: ErrorAppDisabled},
		{name: "credential", deps: Dependencies{Apps: appRepositoryStub{app: &wechatapp.WechatApp{Type: wechatapp.MiniProgram, Status: wechatapp.StatusEnabled}}, Vault: vaultStub{}, Exchanger: &exchangerStub{}}, want: ErrorCredentialMissing},
		{name: "decrypt", deps: Dependencies{Apps: appRepositoryStub{app: enabledApp(wechatapp.MiniProgram)}, Vault: vaultStub{err: errors.New("decrypt")}, Exchanger: &exchangerStub{}}, want: ErrorSecretDecryptFailed},
		{name: "exchange", deps: Dependencies{Apps: appRepositoryStub{app: enabledApp(wechatapp.MiniProgram)}, Vault: vaultStub{plain: []byte("secret")}, Exchanger: &exchangerStub{err: errors.New("provider")}}, want: ErrorProviderExchange},
		{name: "invalid response", deps: Dependencies{Apps: appRepositoryStub{app: enabledApp(wechatapp.MiniProgram)}, Vault: vaultStub{plain: []byte("secret")}, Exchanger: &exchangerStub{empty: true}}, want: ErrorInvalidProviderReply},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(tt.deps)
			_, err := resolver.Resolve(context.Background(), ResolveRequest{
				Provider: domain.ProviderWechatMinip,
				Realm:    "app-1",
				Code:     "code-1",
			})
			requireErrorKind(t, err, tt.want)
		})
	}
}

func TestResolverRequiresWecomAgentConfiguration(t *testing.T) {
	resolver := NewResolver(Dependencies{
		Apps:      appRepositoryStub{app: enabledApp(wechatapp.MP)},
		Vault:     vaultStub{plain: []byte("secret")},
		Exchanger: &exchangerStub{},
	})

	_, err := resolver.Resolve(context.Background(), ResolveRequest{
		Provider: domain.ProviderWecom,
		Realm:    "corp-1",
		Code:     "code-1",
	})
	requireErrorKind(t, err, ErrorProviderConfig)
}

func TestResolverOwnsWecomConfigurationAndProviderExchange(t *testing.T) {
	exchanger := &exchangerStub{}
	resolver := NewResolver(Dependencies{
		Apps:      appRepositoryStub{app: enabledApp(wechatapp.MP)},
		Vault:     vaultStub{plain: []byte("corp-secret")},
		Exchanger: exchanger,
		Config:    Config{WeComAgentID: " agent-1 "},
	})

	identity, err := resolver.Resolve(context.Background(), ResolveRequest{
		Provider: domain.ProviderWecom,
		Realm:    "corp-1",
		Code:     "auth-code",
	})

	require.NoError(t, err)
	require.Equal(t, 1, exchanger.wecomCalls)
	require.Equal(t, "corp-1", exchanger.corpID)
	require.Equal(t, "agent-1", exchanger.agentID)
	require.Equal(t, "corp-secret", exchanger.secret)
	require.Equal(t, "auth-code", exchanger.code)
	userID, ok := identity.Identifier(domain.IdentifierUserID)
	require.True(t, ok)
	require.Equal(t, "user-1", userID)
}

func requireErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	require.Error(t, err)
	kind, ok := KindOf(err)
	require.True(t, ok)
	require.Equal(t, want, kind)
}

func enabledApp(appType wechatapp.AppType) *wechatapp.WechatApp {
	return &wechatapp.WechatApp{
		Type:   appType,
		Status: wechatapp.StatusEnabled,
		Cred: &wechatapp.Credentials{Auth: &wechatapp.AuthSecret{
			AppSecretCipher: []byte("cipher"),
		}},
	}
}

func disabledApp() *wechatapp.WechatApp {
	app := enabledApp(wechatapp.MiniProgram)
	app.Status = wechatapp.StatusDisabled
	return app
}

type appRepositoryStub struct {
	app *wechatapp.WechatApp
	err error
}

func (s appRepositoryStub) Create(context.Context, *wechatapp.WechatApp) error { return nil }
func (s appRepositoryStub) GetByID(context.Context, idutil.ID) (*wechatapp.WechatApp, error) {
	return s.app, s.err
}
func (s appRepositoryStub) GetByAppID(context.Context, string) (*wechatapp.WechatApp, error) {
	return s.app, s.err
}
func (s appRepositoryStub) List(context.Context, wechatapp.ListFilter) ([]*wechatapp.WechatApp, error) {
	return nil, nil
}
func (s appRepositoryStub) Update(context.Context, *wechatapp.WechatApp) error { return nil }

type vaultStub struct {
	plain []byte
	err   error
}

func (s vaultStub) Encrypt(context.Context, []byte) ([]byte, error)      { return nil, nil }
func (s vaultStub) Decrypt(context.Context, []byte) ([]byte, error)      { return s.plain, s.err }
func (s vaultStub) Sign(context.Context, string, []byte) ([]byte, error) { return nil, nil }

type exchangerStub struct {
	err        error
	empty      bool
	miniCalls  int
	openCalls  int
	wecomCalls int
	corpID     string
	agentID    string
	secret     string
	code       string
}

func (s *exchangerStub) ExchangeWxMinipCode(context.Context, string, string, string) (string, string, error) {
	s.miniCalls++
	if s.empty {
		return "", "", s.err
	}
	return "open-1", "union-1", s.err
}
func (s *exchangerStub) ExchangeWxOpenCode(context.Context, string, string, string) (string, string, error) {
	s.openCalls++
	if s.empty {
		return "", "", s.err
	}
	return "open-1", "union-1", s.err
}
func (s *exchangerStub) ExchangeWecomCode(_ context.Context, corpID, agentID, secret, code string) (string, string, error) {
	s.wecomCalls++
	s.corpID = corpID
	s.agentID = agentID
	s.secret = secret
	s.code = code
	if s.empty {
		return "", "", s.err
	}
	return "open-user-1", "user-1", s.err
}
