package signin

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credentialapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/credential"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestSignInPreservesAuthenticationGrantIssuerErrorCodes(t *testing.T) {
	tests := []struct {
		name      string
		issueCode int
	}{
		{name: "issued"},
		{name: "blocked user", issueCode: code.ErrUserBlocked},
		{name: "disabled identity", issueCode: code.ErrLoginIdentityDisabled},
		{name: "inactive user", issueCode: code.ErrUserInactive},
		{name: "admission evaluation failure", issueCode: code.ErrInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal := &authentication.Principal{
				UserID:          meta.FromUint64(1),
				LoginIdentityID: meta.FromUint64(2),
				TenantID:        meta.FromUint64(3),
			}
			grantIssuer := &authenticationGrantIssuerStub{errCode: tt.issueCode}
			strategy := signInStrategyStub{decision: authentication.AuthDecision{OK: true, Principal: principal}}
			usecase := New(Dependencies{
				AuthenticationGrantIssuer: grantIssuer,
				MethodRegistry:            signInMethodRegistryStub{},
				ProofFactory:              signInProofFactoryStub{},
				Authenticator:             authentication.NewAuthenticator(strategy),
			})

			result, err := usecase.Execute(context.Background(), method.LoginRequest{})
			if tt.issueCode == 0 {
				if err != nil || result == nil {
					t.Fatalf("Execute() result = %#v, err = %v", result, err)
				}
			} else {
				if got := perrors.ParseCoder(err).Code(); got != tt.issueCode {
					t.Fatalf("Execute() error code = %d, want %d, err = %v", got, tt.issueCode, err)
				}
				if result != nil {
					t.Fatalf("Execute() result = %#v, want nil", result)
				}
			}
			if !grantIssuer.called {
				t.Fatal("IssueAuthentication() was not called")
			}
		})
	}
}

func TestSignInRecordsCredentialBeforeIssuingAuthenticationGrant(t *testing.T) {
	t.Parallel()

	principal := &authentication.Principal{
		UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2), TenantID: meta.FromUint64(3),
	}
	order := make([]string, 0, 2)
	grantIssuer := &authenticationGrantIssuerStub{order: &order}
	usecase := New(Dependencies{
		AuthenticationGrantIssuer: grantIssuer,
		MethodRegistry:            signInMethodRegistryStub{},
		ProofFactory:              signInProofFactoryStub{},
		Authenticator: authentication.NewAuthenticator(signInStrategyStub{decision: authentication.AuthDecision{
			OK: true, Principal: principal, CredentialID: meta.FromUint64(4),
		}}),
		CredentialRecorder: credentialRecorderStub{order: &order},
	})

	result, err := usecase.Execute(context.Background(), method.LoginRequest{})

	if err != nil || result == nil {
		t.Fatalf("Execute() result = %#v, err = %v", result, err)
	}
	if len(order) != 2 || order[0] != "record" || order[1] != "issue" {
		t.Fatalf("call order = %v, want [record issue]", order)
	}
}

func TestSignInProviderExchangeFailureKeepsPublicContract(t *testing.T) {
	resolutionErr := &idpresolver.ResolutionError{
		Kind:     idpresolver.ErrorProviderExchange,
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    "mini-app",
	}
	proofErr := authnexternal.MapLoginProofError(context.Background(), resolutionErr, "wechat_minip")
	usecase := New(Dependencies{
		MethodRegistry: signInMethodRegistryStub{},
		ProofFactory:   signInProofFactoryErrorStub{err: proofErr},
	})

	credential, err := usecase.buildCredential(context.Background(), method.LoginRequest{})
	if credential != nil {
		t.Fatalf("buildCredential() credential = %#v, want nil", credential)
	}
	coder := perrors.ParseCoder(err)
	if got := coder.Code(); got != code.ErrInternalServerError {
		t.Fatalf("buildCredential() error code = %d, want %d, err = %v", got, code.ErrInternalServerError, err)
	}
	if got := coder.String(); got != "Internal server error" {
		t.Fatalf("buildCredential() public message = %q, want %q", got, "Internal server error")
	}
}

type signInCredentialStub struct{}

func (signInCredentialStub) CredentialKind() authentication.CredentialKind {
	return authentication.CredentialKindPassword
}

type signInMethodRegistryStub struct{}

func (signInMethodRegistryStub) Select(context.Context, method.LoginRequest) (method.LoginMethodSelection, error) {
	return method.LoginMethodSelection{CredentialKind: method.CredentialKindPassword}, nil
}

type signInProofFactoryStub struct{}

func (signInProofFactoryStub) Build(context.Context, method.LoginMethodSelection) (authentication.AuthCredential, error) {
	return signInCredentialStub{}, nil
}

type signInProofFactoryErrorStub struct {
	err error
}

func (s signInProofFactoryErrorStub) Build(context.Context, method.LoginMethodSelection) (authentication.AuthCredential, error) {
	return nil, s.err
}

type signInStrategyStub struct {
	decision authentication.AuthDecision
}

func (signInStrategyStub) Kind() authentication.CredentialKind {
	return authentication.CredentialKindPassword
}

func (s signInStrategyStub) Authenticate(context.Context, authentication.AuthCredential) (authentication.AuthDecision, error) {
	return s.decision, nil
}

type credentialRecorderStub struct {
	order *[]string
}

var _ credentialapp.Recorder = credentialRecorderStub{}

func (s credentialRecorderStub) Record(context.Context, authentication.AuthDecision) error {
	*s.order = append(*s.order, "record")
	return nil
}

type authenticationGrantIssuerStub struct {
	called  bool
	errCode int
	order   *[]string
}

func (s *authenticationGrantIssuerStub) IssueAuthentication(_ context.Context, principal *authentication.Principal) (*tokenapp.TokenPair, error) {
	s.called = true
	if s.order != nil {
		*s.order = append(*s.order, "issue")
	}
	if s.errCode != 0 {
		return nil, perrors.WithCode(s.errCode, "authentication grant denied")
	}
	return tokenapp.NewTokenPair(
		tokenapp.NewAccessToken("a", "access", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, time.Minute),
		tokenapp.NewRefreshToken("r", "refresh", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, nil, nil, time.Hour),
	), nil
}
