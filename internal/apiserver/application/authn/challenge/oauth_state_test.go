package challenge

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestWechatOpenOAuthStateCreatesChallengeWithPayloadAndConsumesOnce(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	creator := newOAuthStateCreator(repo, time.Minute)
	verifier := newOAuthStateVerifier(repo)
	ctx := context.Background()

	created, err := creator.StartWechatOpenLogin(ctx, StartWechatOpenLoginInput{
		AppID:       "wx-app",
		RedirectURI: "https://example.com/callback",
		Nonce:       "nonce-1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.State)

	stored := repo.items[oauthStateChallengeID(SceneWechatOpenLogin, created.State)]
	require.NotNil(t, stored)
	require.Equal(t, challengeDomain.TypeOAuthState, stored.Type)
	require.Equal(t, SceneWechatOpenLogin, stored.Scene)
	require.Equal(t, "wx-app", stored.Payload[PayloadKeyAppID])
	require.Equal(t, "https://example.com/callback", stored.Payload[PayloadKeyRedirectURI])
	require.Equal(t, "nonce-1", stored.Payload[PayloadKeyNonce])

	got, err := verifier.VerifyAndConsumeWechatOpenLogin(ctx, created.State)
	require.NoError(t, err)
	require.Equal(t, "wx-app", got.AppID)
	require.Equal(t, "https://example.com/callback", got.RedirectURI)
	require.Equal(t, "nonce-1", got.Nonce)

	_, err = verifier.VerifyAndConsumeWechatOpenLogin(ctx, created.State)
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
}

func TestWechatOpenOAuthStateRejectsMissingExpiredAndMismatched(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	verifier := newOAuthStateVerifier(repo)
	ctx := context.Background()

	_, err := verifier.VerifyAndConsumeWechatOpenLogin(ctx, "")
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())

	_, err = verifier.VerifyAndConsumeWechatOpenLogin(ctx, "missing-state")
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())

	expired := &challengeDomain.AuthChallenge{
		ID:         oauthStateChallengeID(SceneWechatOpenLogin, "expired"),
		Type:       challengeDomain.TypeOAuthState,
		Scene:      SceneWechatOpenLogin,
		Target:     "wx-app",
		SecretHash: oauthStateSecretHash("expired"),
		Payload:    map[string]string{PayloadKeyAppID: "wx-app"},
		ExpiresAt:  time.Now().Add(-time.Minute),
		CreatedAt:  time.Now().Add(-2 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, expired))
	_, err = verifier.VerifyAndConsumeWechatOpenLogin(ctx, "expired")
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
}

func TestWechatOpenLinkOAuthStateCarriesUserIDAndIsSceneIsolated(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	creator := newOAuthStateCreator(repo, time.Minute)
	verifier := newOAuthStateVerifier(repo)
	ctx := context.Background()

	created, err := creator.StartWechatOpenLink(ctx, StartWechatOpenLinkInput{
		AppID:       "wx-app",
		RedirectURI: "/account/security",
		UserID:      meta.FromUint64(42),
	})
	require.NoError(t, err)

	stored := repo.items[oauthStateChallengeID(SceneWechatOpenLink, created.State)]
	require.NotNil(t, stored)
	require.Equal(t, SceneWechatOpenLink, stored.Scene)
	require.Equal(t, "42", stored.Payload[PayloadKeyUserID])

	// 绑定场景的 state 不能被登录场景消费（scene 隔离，spec 8.3.4）。
	_, err = verifier.VerifyAndConsumeWechatOpenLogin(ctx, created.State)
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())

	got, err := verifier.VerifyAndConsumeWechatOpenLink(ctx, created.State)
	require.NoError(t, err)
	require.Equal(t, "wx-app", got.AppID)
	require.Equal(t, "/account/security", got.RedirectURI)
	require.Equal(t, meta.FromUint64(42), got.UserID)

	// 一次性消费。
	_, err = verifier.VerifyAndConsumeWechatOpenLink(ctx, created.State)
	require.Error(t, err)
}

func TestWechatOpenLinkRequiresUserID(t *testing.T) {
	t.Parallel()

	creator := newOAuthStateCreator(newChallengeRepoStub(), time.Minute)
	_, err := creator.StartWechatOpenLink(context.Background(), StartWechatOpenLinkInput{
		AppID:       "wx-app",
		RedirectURI: "/account/security",
	})
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}
