package linking

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type fakeLinkStateStarter struct {
	gotApp, gotRedirect, gotNonce string
	gotUser                       meta.ID
	result                        WechatOpenLinkState
	err                           error
}

func (f *fakeLinkStateStarter) StartWechatOpenLink(_ context.Context, appID, redirectURI string, userID meta.ID, nonce string) (WechatOpenLinkState, error) {
	f.gotApp, f.gotRedirect, f.gotUser, f.gotNonce = appID, redirectURI, userID, nonce
	return f.result, f.err
}

type fakeLinkStateVerifier struct {
	ctx WechatOpenLinkContext
	err error
}

func (f *fakeLinkStateVerifier) VerifyAndConsumeWechatOpenLink(context.Context, string) (WechatOpenLinkContext, error) {
	return f.ctx, f.err
}

type fakeURLBuilder struct{ gotApp, gotRedirect, gotState string }

func (f *fakeURLBuilder) BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error) {
	f.gotApp, f.gotRedirect, f.gotState = appID, redirectURI, state
	return "https://open.weixin.qq.com/connect/qrconnect?appid=" + appID + "&state=" + state, nil
}

type fakeLinker struct {
	gotReq LinkRequest
	result *LinkResult
	err    error
}

func (f *fakeLinker) List(context.Context, meta.ID) ([]LoginIdentityView, error) { return nil, nil }
func (f *fakeLinker) Unlink(context.Context, UnlinkCommand) error                { return nil }
func (f *fakeLinker) Link(_ context.Context, req LinkRequest) (*LinkResult, error) {
	f.gotReq = req
	return f.result, f.err
}

func TestStartWechatOpenLinkAuthorizeBuildsState(t *testing.T) {
	starter := &fakeLinkStateStarter{result: WechatOpenLinkState{State: "st", Nonce: "nc", ExpiresAt: time.Now().Add(time.Minute)}}
	urlb := &fakeURLBuilder{}
	uc := NewStartWechatOpenLinkAuthorize(starter, urlb)

	res, err := uc.Execute(context.Background(), StartWechatOpenLinkAuthorizeInput{
		UserID:      meta.FromUint64(7),
		AppID:       "wx-app",
		RedirectURI: "https://iam.example.com/cb",
	})

	require.NoError(t, err)
	require.Equal(t, "st", res.State)
	require.Contains(t, res.AuthorizeURL, "state=st")
	require.Equal(t, meta.FromUint64(7), starter.gotUser)
	require.Equal(t, "wx-app", starter.gotApp)
	require.Equal(t, "wx-app", urlb.gotApp)
	require.Equal(t, "https://iam.example.com/cb", urlb.gotRedirect)
}

func TestStartWechatOpenLinkAuthorizeRequiresUserID(t *testing.T) {
	uc := NewStartWechatOpenLinkAuthorize(&fakeLinkStateStarter{}, &fakeURLBuilder{})
	_, err := uc.Execute(context.Background(), StartWechatOpenLinkAuthorizeInput{AppID: "wx-app", RedirectURI: "/cb"})
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestCompleteWechatOpenLinkBindsToStateUser(t *testing.T) {
	verifier := &fakeLinkStateVerifier{ctx: WechatOpenLinkContext{AppID: "wx-app", UserID: meta.FromUint64(7)}}
	linker := &fakeLinker{result: &LinkResult{Identity: &loginidentity.LoginIdentity{ID: meta.FromUint64(1000)}}}
	uc := NewCompleteWechatOpenLink(verifier, linker)

	res, err := uc.Execute(context.Background(), CompleteWechatOpenLinkInput{
		State:          "st",
		Code:           "scan-code",
		ExpectedUserID: meta.FromUint64(7),
	})

	require.NoError(t, err)
	require.Equal(t, meta.FromUint64(1000), res.Identity.ID)
	require.Equal(t, meta.FromUint64(7), linker.gotReq.UserID)
	input, ok := linker.gotReq.Input.(LinkWechatOpenInput)
	require.True(t, ok)
	require.Equal(t, "wx-app", input.AppID)
	require.Equal(t, "scan-code", input.Code)
}

func TestCompleteWechatOpenLinkRejectsUserMismatch(t *testing.T) {
	verifier := &fakeLinkStateVerifier{ctx: WechatOpenLinkContext{AppID: "wx-app", UserID: meta.FromUint64(7)}}
	linker := &fakeLinker{}
	uc := NewCompleteWechatOpenLink(verifier, linker)

	_, err := uc.Execute(context.Background(), CompleteWechatOpenLinkInput{
		State:          "st",
		Code:           "scan-code",
		ExpectedUserID: meta.FromUint64(99),
	})

	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
	require.Equal(t, LinkRequest{}, linker.gotReq, "must not bind when state belongs to another user")
}

func TestCompleteWechatOpenLinkRejectsStateWithoutUserID(t *testing.T) {
	verifier := &fakeLinkStateVerifier{ctx: WechatOpenLinkContext{AppID: "wx-app"}}
	uc := NewCompleteWechatOpenLink(verifier, &fakeLinker{})

	_, err := uc.Execute(context.Background(), CompleteWechatOpenLinkInput{State: "st", Code: "c"})
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
}

func TestCompleteWechatOpenLinkPropagatesStateError(t *testing.T) {
	verifier := &fakeLinkStateVerifier{err: errors.New("state gone")}
	uc := NewCompleteWechatOpenLink(verifier, &fakeLinker{})

	_, err := uc.Execute(context.Background(), CompleteWechatOpenLinkInput{State: "st", Code: "c"})
	require.Error(t, err)
}
