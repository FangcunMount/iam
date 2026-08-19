package externalidentity

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestResolverAppFailuresKeepUseCaseSpecificCodes(t *testing.T) {
	tests := []struct {
		name  string
		err   *idpresolver.ResolutionError
		login int
		link  int
		sign  int
	}{
		{name: "query", err: resolutionError(idpresolver.ErrorAppQueryFailed), login: code.ErrProofBuildFailed, link: code.ErrInvalidArgument, sign: code.ErrInvalidArgument},
		{name: "not found", err: resolutionError(idpresolver.ErrorAppNotFound), login: code.ErrProofBuildFailed, link: code.ErrInvalidArgument, sign: code.ErrInvalidArgument},
		{name: "disabled", err: resolutionError(idpresolver.ErrorAppDisabled), login: code.ErrProofBuildFailed, link: code.ErrInvalidArgument, sign: code.ErrInvalidArgument},
		{name: "credential missing", err: resolutionError(idpresolver.ErrorCredentialMissing), login: code.ErrProofBuildFailed, link: code.ErrInvalidArgument, sign: code.ErrInvalidArgument},
		{name: "decrypt", err: resolutionError(idpresolver.ErrorSecretDecryptFailed), login: code.ErrProofBuildFailed, link: code.ErrInvalidArgument, sign: code.ErrInvalidArgument},
		{
			name: "type mismatch",
			err: &idpresolver.ResolutionError{
				Kind:     idpresolver.ErrorAppTypeMismatch,
				Provider: idpidentity.ProviderWechatMinip,
				Realm:    "app-1",
			},
			login: code.ErrWechatAppTypeMismatch,
			link:  code.ErrWechatAppTypeMismatch,
			sign:  code.ErrWechatAppTypeMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginErr := MapLoginProofError(context.Background(), tt.err, "wechat_minip")
			linkErr := MapLinkingError(tt.err)
			signupErr := MapSignupError(tt.err)

			require.Equal(t, tt.login, perrors.ParseCoder(loginErr).Code())
			require.Equal(t, tt.link, perrors.ParseCoder(linkErr).Code())
			require.Equal(t, tt.sign, perrors.ParseCoder(signupErr).Code())
		})
	}
}

func TestProviderExchangeFailureKeepsAuthenticationStage(t *testing.T) {
	err := resolutionError(idpresolver.ErrorProviderExchange)

	mapped := MapLoginProofError(context.Background(), err, "wechat_minip")
	cause, ok := AuthenticationCause(mapped)

	require.True(t, ok)
	require.Contains(t, cause.Error(), "failed to exchange wx minip code")
	require.ErrorIs(t, mapped, err)
}

func resolutionError(kind idpresolver.ErrorKind) *idpresolver.ResolutionError {
	return &idpresolver.ResolutionError{
		Kind:     kind,
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    "app-1",
	}
}
