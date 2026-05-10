package token

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSessionTokenPairIssuerWritesStableAuthTime(t *testing.T) {
	t.Parallel()

	codec := &accessTokenCodecStub{}
	store := &tokenStoreStub{}
	issuer := newSessionTokenPairIssuer(codec, store, NewStringClaimMapper(), time.Minute, time.Hour)
	issuedAt := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	pair, err := issuer.IssueTokenPair(context.Background(), &authentication.Principal{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		Claims:          map[string]any{"auth_time": issuedAt},
	}, &sessiondomain.Session{SessionID: "session-1"})

	require.NoError(t, err)
	require.NotNil(t, pair)
	require.Equal(t, issuedAt.Format(time.RFC3339), codec.principal.Claims["auth_time"])
	require.Equal(t, issuedAt.Format(time.RFC3339), store.refresh.SessionClaims["auth_time"])
}

func TestSessionTokenPairIssuerDefaultsAuthTimeWhenMissing(t *testing.T) {
	t.Parallel()

	codec := &accessTokenCodecStub{}
	store := &tokenStoreStub{}
	issuer := newSessionTokenPairIssuer(codec, store, NewStringClaimMapper(), time.Minute, time.Hour)

	_, err := issuer.IssueTokenPair(context.Background(), &authentication.Principal{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
	}, &sessiondomain.Session{SessionID: "session-1"})

	require.NoError(t, err)
	authTime, ok := codec.principal.Claims["auth_time"].(string)
	require.True(t, ok)
	_, err = time.Parse(time.RFC3339, authTime)
	require.NoError(t, err)
	require.Equal(t, authTime, store.refresh.SessionClaims["auth_time"])
}

type accessTokenCodecStub struct {
	principal *Principal
}

func (s *accessTokenCodecStub) IssueAccessToken(_ context.Context, principal *Principal, expiresIn time.Duration) (*Token, error) {
	s.principal = principal
	return NewAccessToken("access-id", "access-token", principal.SessionID, principal.UserID, principal.LoginIdentityID, principal.TenantID, expiresIn), nil
}

func (s *accessTokenCodecStub) IssueServiceToken(context.Context, string, []string, map[string]string, time.Duration) (*Token, error) {
	return nil, nil
}

func (s *accessTokenCodecStub) VerifyAccessToken(context.Context, string) (*TokenClaims, error) {
	return nil, nil
}

type tokenStoreStub struct {
	refresh *Token
}

func (s *tokenStoreStub) SaveRefreshToken(_ context.Context, token *Token) error {
	s.refresh = token
	return nil
}

func (s *tokenStoreStub) GetRefreshToken(context.Context, string) (*Token, error) {
	return nil, nil
}

func (s *tokenStoreStub) DeleteRefreshToken(context.Context, string) error {
	return nil
}

func (s *tokenStoreStub) MarkAccessTokenRevoked(context.Context, string, time.Duration) error {
	return nil
}

func (s *tokenStoreStub) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return false, nil
}
