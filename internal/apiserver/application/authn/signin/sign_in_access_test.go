package signin

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestSignInChecksAdmissionBeforeIssuingTokens(t *testing.T) {
	tests := []struct {
		name     string
		reason   admissiondomain.DenialReason
		checkErr error
		wantCode int
		wantCall bool
	}{
		{name: "active", wantCall: true},
		{name: "blocked user", reason: admissiondomain.ReasonUserBlocked, wantCode: code.ErrUserBlocked},
		{name: "disabled identity", reason: admissiondomain.ReasonLoginIdentityDisabled, wantCode: code.ErrLoginIdentityDisabled},
		{name: "inactive user", reason: admissiondomain.ReasonUserInactive, wantCode: code.ErrUserInactive},
		{name: "checker failure", checkErr: errors.New("database unavailable"), wantCode: code.ErrInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal := &authentication.Principal{
				UserID:          meta.FromUint64(1),
				LoginIdentityID: meta.FromUint64(2),
				TenantID:        meta.FromUint64(3),
			}
			sessionEstablisher := &sessionEstablisherStub{}
			strategy := signInStrategyStub{decision: authentication.AuthDecision{OK: true, Principal: principal}}
			usecase := New(Dependencies{
				SessionEstablisher: sessionEstablisher,
				MethodRegistry:     signInMethodRegistryStub{},
				ProofFactory:       signInProofFactoryStub{},
				Authenticator:      authentication.NewAuthenticator(strategy),
				AdmissionPolicy:    signInAdmissionPolicyStub{reason: tt.reason, err: tt.checkErr},
			})

			result, err := usecase.Execute(context.Background(), method.LoginRequest{})
			if tt.wantCode == 0 {
				if err != nil || result == nil {
					t.Fatalf("Execute() result = %#v, err = %v", result, err)
				}
			} else {
				if got := perrors.ParseCoder(err).Code(); got != tt.wantCode {
					t.Fatalf("Execute() error code = %d, want %d, err = %v", got, tt.wantCode, err)
				}
				if result != nil {
					t.Fatalf("Execute() result = %#v, want nil", result)
				}
			}
			if sessionEstablisher.called != tt.wantCall {
				t.Fatalf("EstablishSession() called = %t, want %t", sessionEstablisher.called, tt.wantCall)
			}
		})
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

type signInAdmissionPolicyStub struct {
	reason admissiondomain.DenialReason
	err    error
}

func (s signInAdmissionPolicyStub) Evaluate(_ context.Context, subject admissiondomain.Subject) (admissiondomain.Decision, error) {
	if s.err != nil {
		return admissiondomain.Decision{}, s.err
	}
	if s.reason != "" {
		return admissiondomain.Deny(subject, s.reason), nil
	}
	return admissiondomain.Admit(subject), nil
}

type sessionEstablisherStub struct {
	called bool
}

func (s *sessionEstablisherStub) EstablishSession(_ context.Context, principal *authentication.Principal) (*tokenapp.TokenPair, error) {
	s.called = true
	return tokenapp.NewTokenPair(
		tokenapp.NewAccessToken("a", "access", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, time.Minute),
		tokenapp.NewRefreshToken("r", "refresh", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, nil, nil, time.Hour),
	), nil
}
