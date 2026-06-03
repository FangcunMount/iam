package assembler

import (
	"context"

	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// wechatOpenLinkStateAdapter adapts the challenge OAuth-state use cases to
// linking's local WeChat-open link ports, so linking does not depend on the
// challenge package directly.
type wechatOpenLinkStateAdapter struct {
	starter  challengeApp.WechatOpenLinkOAuthStateStarter
	verifier challengeApp.WechatOpenLinkOAuthStateVerifier
}

func newWechatOpenLinkStateAdapter(
	starter challengeApp.WechatOpenLinkOAuthStateStarter,
	verifier challengeApp.WechatOpenLinkOAuthStateVerifier,
) *wechatOpenLinkStateAdapter {
	return &wechatOpenLinkStateAdapter{starter: starter, verifier: verifier}
}

var (
	_ linkingApp.WechatOpenLinkStateStarter  = (*wechatOpenLinkStateAdapter)(nil)
	_ linkingApp.WechatOpenLinkStateVerifier = (*wechatOpenLinkStateAdapter)(nil)
)

func (a *wechatOpenLinkStateAdapter) StartWechatOpenLink(ctx context.Context, appID, redirectURI string, userID meta.ID, nonce string) (linkingApp.WechatOpenLinkState, error) {
	created, err := a.starter.StartWechatOpenLink(ctx, challengeApp.StartWechatOpenLinkInput{
		AppID:       appID,
		RedirectURI: redirectURI,
		UserID:      userID,
		Nonce:       nonce,
	})
	if err != nil {
		return linkingApp.WechatOpenLinkState{}, err
	}
	return linkingApp.WechatOpenLinkState{
		State:     created.State,
		Nonce:     created.Nonce,
		ExpiresAt: created.ExpiresAt,
	}, nil
}

func (a *wechatOpenLinkStateAdapter) VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (linkingApp.WechatOpenLinkContext, error) {
	stateCtx, err := a.verifier.VerifyAndConsumeWechatOpenLink(ctx, state)
	if err != nil {
		return linkingApp.WechatOpenLinkContext{}, err
	}
	return linkingApp.WechatOpenLinkContext{
		AppID:  stateCtx.AppID,
		UserID: stateCtx.UserID,
	}, nil
}
