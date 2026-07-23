package signin

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestSignInChecksSubjectAccessBeforeIssuingTokens(t *testing.T) {
	tests := []struct {
		name     string
		status   sessiondomain.SubjectAccessStatus
		checkErr error
		wantCode int
		wantCall bool
	}{
		{name: "active", status: sessiondomain.SubjectAccessActive, wantCall: true},
		{name: "blocked user", status: sessiondomain.SubjectAccessBlocked, wantCode: code.ErrUserBlocked},
		{name: "disabled identity", status: sessiondomain.SubjectAccessDisabled, wantCode: code.ErrLoginIdentityDisabled},
		{name: "inactive user", status: sessiondomain.SubjectAccessInactive, wantCode: code.ErrUserInactive},
		{name: "checker failure", checkErr: errors.New("database unavailable"), wantCode: code.ErrInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal := &authentication.Principal{
				UserID:          meta.FromUint64(1),
				LoginIdentityID: meta.FromUint64(2),
				TenantID:        meta.FromUint64(3),
			}
			tokenService := &signInTokenServiceStub{}
			strategy := signInStrategyStub{decision: authentication.AuthDecision{OK: true, Principal: principal}}
			usecase := New(Dependencies{
				TokenService:   tokenService,
				MethodRegistry: signInMethodRegistryStub{},
				ProofFactory:   signInProofFactoryStub{},
				Authenticator:  authentication.NewAuthenticator(strategy),
				AccessChecker:  signInAccessCheckerStub{status: tt.status, err: tt.checkErr},
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
			if tokenService.called != tt.wantCall {
				t.Fatalf("IssueToken() called = %t, want %t", tokenService.called, tt.wantCall)
			}
		})
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

type signInStrategyStub struct {
	decision authentication.AuthDecision
}

func (signInStrategyStub) Kind() authentication.CredentialKind {
	return authentication.CredentialKindPassword
}

func (s signInStrategyStub) Authenticate(context.Context, authentication.AuthCredential) (authentication.AuthDecision, error) {
	return s.decision, nil
}

type signInAccessCheckerStub struct {
	status sessiondomain.SubjectAccessStatus
	err    error
}

func (s signInAccessCheckerStub) Evaluate(context.Context, meta.ID, meta.ID) (sessiondomain.SubjectAccessDecision, error) {
	return sessiondomain.SubjectAccessDecision{Status: s.status}, s.err
}

type signInTokenServiceStub struct {
	called bool
}

func (s *signInTokenServiceStub) IssueToken(_ context.Context, principal *authentication.Principal) (*tokenapp.TokenPair, error) {
	s.called = true
	return tokenapp.NewTokenPair(
		tokenapp.NewAccessToken("a", "access", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, time.Minute),
		tokenapp.NewRefreshToken("r", "refresh", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, nil, nil, time.Hour),
	), nil
}

func (*signInTokenServiceStub) IssueServiceToken(context.Context, tokenapp.IssueServiceTokenRequest) (*tokenapp.TokenIssueResult, error) {
	return nil, nil
}
func (*signInTokenServiceStub) RefreshToken(context.Context, string) (*tokenapp.TokenRefreshResult, error) {
	return nil, nil
}
func (*signInTokenServiceStub) RevokeAccessToken(context.Context, string) error  { return nil }
func (*signInTokenServiceStub) RevokeRefreshToken(context.Context, string) error { return nil }
func (*signInTokenServiceStub) VerifyToken(context.Context, tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error) {
	return nil, nil
}
