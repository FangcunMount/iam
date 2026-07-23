package proof_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestWechatScanBuilderRejectsReusedOAuthState(t *testing.T) {
	t.Parallel()

	repo := newOAuthChallengeRepoStub()
	oauthStates := challengeApp.NewService(repo, challengeApp.SMSOTPDelivery{}, challengeApp.NewCreator(repo), challengeApp.NewVerifier(repo))
	created, err := oauthStates.StartWechatOpenLogin(context.Background(), challengeApp.StartWechatOpenLoginInput{
		AppID:       "wx-app",
		RedirectURI: "https://example.com/callback",
	})
	require.NoError(t, err)

	factory := proof.MustFactory(oauthScanProofBuilder{oauthStates: oauthStates})
	_, err = factory.Build(context.Background(), method.LoginMethodSelection{
		CredentialKind: method.CredentialKindWechatScan,
		Payload: method.WechatScanPayload{
			AppID: "wx-app",
			Code:  "wx-code",
			State: created.State,
		},
	})
	require.NoError(t, err)

	_, err = factory.Build(context.Background(), method.LoginMethodSelection{
		CredentialKind: method.CredentialKindWechatScan,
		Payload: method.WechatScanPayload{
			AppID: "wx-app",
			Code:  "wx-code",
			State: created.State,
		},
	})
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
}

type oauthScanProofBuilder struct {
	oauthStates challengeApp.WechatOpenOAuthStateVerifier
}

func (b oauthScanProofBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWechatScan
}

func (b oauthScanProofBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	scanPayload, ok := payload.(method.WechatScanPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wechat scan payload")
	}
	if _, err := b.oauthStates.VerifyAndConsumeWechatOpenLogin(ctx, scanPayload.State); err != nil {
		return nil, err
	}
	return authentication.NewWechatOpenCredential(authentication.WechatOpenProofSpec{
		TenantID:  common.TenantID,
		AppID:     scanPayload.AppID,
		AppSecret: "secret",
		Code:      scanPayload.Code,
	})
}

type oauthChallengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newOAuthChallengeRepoStub() *oauthChallengeRepoStub {
	return &oauthChallengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *oauthChallengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *oauthChallengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *oauthChallengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, _ []byte) (bool, error) {
	if _, ok := s.items[id]; !ok {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *oauthChallengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
