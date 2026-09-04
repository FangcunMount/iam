package token

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestServiceTokenOnlineRevocationIsEnforced(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()
	claims, err := NewVerifiedServiceClaims(VerifiedTokenClaims{
		TokenID: "service-jti", TokenType: TokenTypeService, Subject: "service:worker",
		Issuer: "https://iam.test", Audience: []string{"qs-api"},
		IssuedAt: now.Add(-time.Minute), NotBefore: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	codec := &bearerTokenCodecStub{claims: claims}
	store := &bearerTokenStoreStub{revoked: make(map[string]bool)}
	sessions := &trackingSessionRevokerStub{}
	verifier := newVerifier(codec, store, nil, nil)
	revoker := newRevoker(codec, store, sessions)

	verified, err := verifier.VerifyToken(ctx, "signed-service-token")
	require.NoError(t, err)
	require.Equal(t, TokenTypeService, verified.TokenType)

	require.NoError(t, revoker.RevokeBearerToken(ctx, "signed-service-token"))
	require.True(t, store.revoked["service-jti"])
	require.Zero(t, sessions.revokeCalls, "service token revocation must not revoke a user session")

	verified, err = verifier.VerifyToken(ctx, "signed-service-token")
	require.Nil(t, verified)
	require.Error(t, err)
	require.Equal(t, code.ErrTokenInvalid, perrors.ParseCoder(err).Code())
}

type bearerTokenCodecStub struct {
	claims *VerifiedTokenClaims
}

func (*bearerTokenCodecStub) IssueAccessToken(context.Context, *AccessTokenSubject, time.Duration) (*AccessToken, error) {
	return nil, nil
}

func (*bearerTokenCodecStub) IssueServiceToken(context.Context, string, []string, map[string]string, time.Duration) (*ServiceToken, error) {
	return nil, nil
}

func (s *bearerTokenCodecStub) VerifyBearerToken(context.Context, string) (*VerifiedTokenClaims, error) {
	return s.claims, nil
}

type bearerTokenStoreStub struct {
	revoked map[string]bool
}

func (*bearerTokenStoreStub) SaveRefreshToken(context.Context, *RefreshToken) error { return nil }

func (*bearerTokenStoreStub) GetRefreshToken(context.Context, string) (*RefreshToken, error) {
	return nil, nil
}

func (*bearerTokenStoreStub) GetConsumedRefreshToken(context.Context, string) (*ConsumedRefreshToken, error) {
	return nil, nil
}

func (*bearerTokenStoreStub) RotateRefreshToken(context.Context, string, string, *RefreshToken) (bool, error) {
	return false, nil
}

func (*bearerTokenStoreStub) DeleteRefreshToken(context.Context, string) error { return nil }

func (s *bearerTokenStoreStub) MarkBearerTokenRevoked(_ context.Context, tokenID string, _ time.Duration) error {
	s.revoked[tokenID] = true
	return nil
}

func (s *bearerTokenStoreStub) IsBearerTokenRevoked(_ context.Context, tokenID string) (bool, error) {
	return s.revoked[tokenID], nil
}

type trackingSessionRevokerStub struct {
	revokeCalls int
}

func (s *trackingSessionRevokerStub) Revoke(context.Context, string, string, string) error {
	s.revokeCalls++
	return nil
}

func (*trackingSessionRevokerStub) RevokeByUser(context.Context, meta.ID, string, string) error {
	return nil
}

func (*trackingSessionRevokerStub) RevokeByLoginIdentity(context.Context, meta.ID, string, string) error {
	return nil
}
