package onboarding

import (
	"context"
	"testing"

	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRequestPreparerBuildsLoginIdentityData(t *testing.T) {
	t.Parallel()

	email, err := meta.NewEmail("mock@example.com")
	require.NoError(t, err)
	openID := "openid-1"
	unionID := "union-1"
	appID := "wx-app"
	tenantID := meta.FromUint64(9001)

	tests := []struct {
		name                   string
		req                    OnboardingRequest
		needPasswordCredential bool
		allowUserRepair        bool
		provider               loginidentity.Provider
		realm                  string
		identifier             string
		globalIdentifier       string
	}{
		{
			name: "opera password",
			req: OnboardingRequest{
				LoginIdentity: UsernameLoginIdentityInput{
					Username:      "zhangsan",
					RealmTenantID: tenantID,
				},
			},
			needPasswordCredential: true,
			provider:               loginidentity.ProviderUsername,
			realm:                  tenantID.String(),
			identifier:             "zhangsan",
		},
		{
			name: "mock consumer password",
			req: OnboardingRequest{
				User:          OnboardingUserInput{Email: email},
				LoginIdentity: MockConsumerUsernameLoginIdentityInput{},
			},
			needPasswordCredential: true,
			provider:               loginidentity.ProviderUsername,
			realm:                  loginidentity.RealmDefault,
			identifier:             email.String(),
		},
		{
			name: "wechat mini",
			req: OnboardingRequest{
				LoginIdentity: WechatMiniLoginIdentityInput{
					AppID:   &appID,
					OpenID:  &openID,
					UnionID: &unionID,
				},
			},
			allowUserRepair:  true,
			provider:         loginidentity.ProviderWechatMinip,
			realm:            appID,
			identifier:       openID,
			globalIdentifier: unionID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prepared, err := newRequestPreparer(nil).Prepare(context.Background(), tt.req)
			require.NoError(t, err)
			require.Equal(t, tt.needPasswordCredential, prepared.LoginIdentity.NeedPasswordCredential)
			require.Equal(t, tt.allowUserRepair, prepared.LoginIdentity.AllowUserRepair)
			require.Equal(t, tt.provider, prepared.LoginIdentity.ProviderKey.Provider)
			require.Equal(t, tt.realm, prepared.LoginIdentity.ProviderKey.Realm)
			require.Equal(t, tt.identifier, prepared.LoginIdentity.ProviderKey.Identifier)
			require.Equal(t, tt.globalIdentifier, prepared.LoginIdentity.ProviderKey.GlobalIdentifier)
		})
	}
}

func TestRequestPreparerRejectsUnsupportedOrIncompleteLoginIdentity(t *testing.T) {
	t.Parallel()

	_, err := newRequestPreparer(nil).Prepare(context.Background(), OnboardingRequest{
		LoginIdentity: nil,
	})
	require.Error(t, err)

	_, err = newRequestPreparer(nil).Prepare(context.Background(), OnboardingRequest{
		LoginIdentity: WechatMiniLoginIdentityInput{},
	})
	require.Error(t, err)
}
