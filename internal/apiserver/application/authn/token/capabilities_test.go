package token

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	grantdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/grant"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestApplicationMapsGrantAdmissionDenial(t *testing.T) {
	t.Parallel()

	app := &application{grantIssuer: grantIssuerStub{err: blockedAdmissionError()}}

	pair, err := app.IssueAuthentication(context.Background(), &authentication.Principal{})

	require.Nil(t, pair)
	require.Equal(t, code.ErrUserBlocked, perrors.ParseCoder(err).Code())
}

func TestApplicationMapsRefreshAdmissionDenial(t *testing.T) {
	t.Parallel()

	app := &application{refresher: refresherStub{err: blockedAdmissionError()}}

	result, err := app.RefreshToken(context.Background(), "refresh-token")

	require.Nil(t, result)
	require.Equal(t, code.ErrUserBlocked, perrors.ParseCoder(err).Code())
}

func TestApplicationMapsVerifyAdmissionDenialToExistingFailureContract(t *testing.T) {
	t.Parallel()

	app := &application{verifier: verifierStub{err: blockedAdmissionError()}}

	result, err := app.VerifyToken(context.Background(), VerifyTokenRequest{AccessToken: "access-token"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Valid)
	require.Equal(t, code.ErrUserBlocked, result.FailureCode)
}

func TestApplicationRejectsDisallowedTokenType(t *testing.T) {
	t.Parallel()

	app := &application{verifier: verifierStub{claims: &tokendomain.VerifiedTokenClaims{
		TokenType: TokenTypeService,
		Issuer:    "https://iam.fangcunmount.cn",
		Audience:  []string{"qs-api"},
	}}}

	result, err := app.VerifyToken(context.Background(), VerifyTokenRequest{
		AccessToken:        "service-token",
		AcceptedTokenTypes: []TokenType{TokenTypeAccess},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Valid)
	require.Nil(t, result.Claims)
}

func TestApplicationDefaultsToAccessTokenType(t *testing.T) {
	t.Parallel()

	app := &application{verifier: verifierStub{claims: &tokendomain.VerifiedTokenClaims{
		TokenType: TokenTypeService,
	}}}
	result, err := app.VerifyToken(context.Background(), VerifyTokenRequest{AccessToken: "service-token"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Valid)
}

type grantIssuerStub struct {
	err error
}

func (s grantIssuerStub) Issue(context.Context, *authentication.Principal) (*grantdomain.AuthenticationGrant, error) {
	return nil, s.err
}

type refresherStub struct {
	err error
}

func (s refresherStub) RefreshToken(context.Context, string) (*tokendomain.UserTokenSet, error) {
	return nil, s.err
}

func (s refresherStub) RevokeRefreshToken(context.Context, string) error {
	return s.err
}

type verifierStub struct {
	err    error
	claims *tokendomain.VerifiedTokenClaims
}

func (s verifierStub) VerifyToken(context.Context, string) (*tokendomain.VerifiedTokenClaims, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.claims, nil
}

func blockedAdmissionError() error {
	subject := admissiondomain.Subject{
		UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2),
	}
	return &admissiondomain.DeniedError{
		Decision: admissiondomain.Deny(subject, admissiondomain.ReasonUserBlocked),
	}
}
