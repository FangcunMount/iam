package handler

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	req "github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authn/request"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSignupResultToResponseUsesNullableCredential(t *testing.T) {
	t.Parallel()

	withoutCredential := signupResultToResponse(&signupApp.SignupResult{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
	})
	require.Nil(t, withoutCredential.Credential)

	withCredential := signupResultToResponse(&signupApp.SignupResult{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		Credential: &signupApp.SignupCredential{
			ID:   meta.FromUint64(3),
			Type: credDomain.CredPassword,
		},
	})
	require.NotNil(t, withCredential.Credential)
	require.Equal(t, uint64(3), withCredential.Credential.ID)
	require.Equal(t, "password", withCredential.Credential.Type)
}

func TestWechatMiniProgramSignupRequestFromHTTPRejectsInvalidOptionalContacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   req.SignUpWithWeChatMiniProgramRequest
	}{
		{
			name: "invalid phone",
			in: req.SignUpWithWeChatMiniProgramRequest{
				Name:   "alice",
				Phone:  "not-a-phone",
				AppID:  "wx-app",
				JsCode: "js-code",
			},
		},
		{
			name: "invalid email",
			in: req.SignUpWithWeChatMiniProgramRequest{
				Name:   "alice",
				Email:  "not-an-email",
				AppID:  "wx-app",
				JsCode: "js-code",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := wechatMiniProgramSignupRequestFromHTTP(tc.in)

			require.Error(t, err)
			require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
		})
	}
}

func TestWechatMiniProgramSignupRequestFromHTTPUsesOnlyCodeProof(t *testing.T) {
	t.Parallel()

	mapped, err := wechatMiniProgramSignupRequestFromHTTP(req.SignUpWithWeChatMiniProgramRequest{
		Name:   "alice",
		AppID:  " wx-app ",
		JsCode: " js-code ",
	})
	require.NoError(t, err)

	identity, ok := mapped.LoginIdentity.(signupApp.WechatMiniLoginIdentityInput)
	require.True(t, ok)
	require.Equal(t, "wx-app", *identity.AppID)
	require.Equal(t, "js-code", *identity.JsCode)
	require.Nil(t, identity.OpenID)
	require.Nil(t, identity.UnionID)
}
