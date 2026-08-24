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

type publicErrorContract struct {
	code    int
	message string
}

func TestResolverAppFailuresKeepUseCaseSpecificPublicContracts(t *testing.T) {
	tests := []struct {
		name  string
		err   *idpresolver.ResolutionError
		login publicErrorContract
		link  publicErrorContract
		sign  publicErrorContract
	}{
		{
			name:  "invalid request",
			err:   resolutionError(idpresolver.ErrorInvalidRequest),
			login: publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "unavailable",
			err:   resolutionError(idpresolver.ErrorUnavailable),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "query",
			err:   resolutionError(idpresolver.ErrorAppQueryFailed),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "not found",
			err:   resolutionError(idpresolver.ErrorAppNotFound),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "disabled",
			err:   resolutionError(idpresolver.ErrorAppDisabled),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "credential missing",
			err:   resolutionError(idpresolver.ErrorCredentialMissing),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name:  "decrypt",
			err:   resolutionError(idpresolver.ErrorSecretDecryptFailed),
			login: publicErrorContract{code: code.ErrProofBuildFailed, message: "Failed to build authentication proof"},
			link:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
			sign:  publicErrorContract{code: code.ErrInvalidArgument, message: "Invalid argument"},
		},
		{
			name: "type mismatch",
			err: &idpresolver.ResolutionError{
				Kind:     idpresolver.ErrorAppTypeMismatch,
				Provider: idpidentity.ProviderWechatMinip,
				Realm:    "app-1",
			},
			login: publicErrorContract{code: code.ErrWechatAppTypeMismatch, message: "Wechat app type does not match"},
			link:  publicErrorContract{code: code.ErrWechatAppTypeMismatch, message: "Wechat app type does not match"},
			sign:  publicErrorContract{code: code.ErrWechatAppTypeMismatch, message: "Wechat app type does not match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginErr := MapLoginProofError(context.Background(), tt.err, "wechat_minip")
			linkErr := MapLinkingError(tt.err)
			signupErr := MapSignupError(tt.err)

			requirePublicErrorContract(t, loginErr, tt.login)
			requirePublicErrorContract(t, linkErr, tt.link)
			requirePublicErrorContract(t, signupErr, tt.sign)
		})
	}
}

func TestProviderExchangeFailureKeepsLinkingAndSignupPublicContracts(t *testing.T) {
	for _, kind := range []idpresolver.ErrorKind{
		idpresolver.ErrorProviderExchange,
		idpresolver.ErrorInvalidProviderReply,
	} {
		t.Run(string(kind), func(t *testing.T) {
			err := resolutionError(kind)

			requirePublicErrorContract(t, MapLinkingError(err), publicErrorContract{
				code:    code.ErrInvalidCredential,
				message: "Invalid credential",
			})
			requirePublicErrorContract(t, MapSignupError(err), publicErrorContract{
				code:    code.ErrInvalidCredential,
				message: "Invalid credential",
			})
		})
	}
}

func TestWecomConfigurationFailureKeepsLoginAndLinkingPublicContracts(t *testing.T) {
	err := &idpresolver.ResolutionError{
		Kind:     idpresolver.ErrorProviderConfig,
		Provider: idpidentity.ProviderWecom,
		Realm:    "corp-1",
	}

	requirePublicErrorContract(t, MapLoginProofError(context.Background(), err, "wecom"), publicErrorContract{
		code:    code.ErrProofBuildFailed,
		message: "Failed to build authentication proof",
	})
	requirePublicErrorContract(t, MapLinkingError(err), publicErrorContract{
		code:    code.ErrInvalidArgument,
		message: "Invalid argument",
	})
}

func TestProviderExchangeFailureKeepsAuthenticationStage(t *testing.T) {
	err := resolutionError(idpresolver.ErrorProviderExchange)

	mapped := MapLoginProofError(context.Background(), err, "wechat_minip")
	cause, ok := AuthenticationCause(mapped)

	require.True(t, ok)
	require.Contains(t, cause.Error(), "failed to exchange wx minip code")
	require.ErrorIs(t, mapped, err)
}

func requirePublicErrorContract(t *testing.T, err error, want publicErrorContract) {
	t.Helper()
	require.Error(t, err)
	coder := perrors.ParseCoder(err)
	require.Equal(t, want.code, coder.Code())
	require.Equal(t, want.message, coder.String())
}

func resolutionError(kind idpresolver.ErrorKind) *idpresolver.ResolutionError {
	return &idpresolver.ResolutionError{
		Kind:     kind,
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    "app-1",
	}
}
