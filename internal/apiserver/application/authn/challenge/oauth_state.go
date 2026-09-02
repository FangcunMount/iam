package challenge

import (
	"context"
	"errors"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

const (
	// SceneWechatOpenLogin 微信开放平台扫码登录场景（user_id 必空）。
	SceneWechatOpenLogin = "wechat_open_login"
	// SceneWechatOpenLink 微信开放平台扫码绑定场景（user_id 必填，来自已登录用户）。
	SceneWechatOpenLink = "wechat_open_link"

	PayloadKeyAppID       = challengeDomain.OAuthPayloadKeyAppID
	PayloadKeyRedirectURI = challengeDomain.OAuthPayloadKeyRedirectURI
	PayloadKeyNonce       = challengeDomain.OAuthPayloadKeyNonce
	PayloadKeyUserID      = challengeDomain.OAuthPayloadKeyUserID

	defaultOAuthStateTTL = challengeDomain.DefaultOAuthStateTTL
)

// StartWechatOpenLoginInput 创建微信开放平台扫码登录 OAuth state 的输入
type StartWechatOpenLoginInput struct {
	AppID       string
	RedirectURI string
	Nonce       string
}

// StartWechatOpenLoginResult 返回给前端的 state 及元数据
type StartWechatOpenLoginResult struct {
	State     string
	Nonce     string
	ExpiresAt time.Time
}

// StartWechatOpenLinkInput 创建微信开放平台扫码绑定 OAuth state 的输入。
type StartWechatOpenLinkInput struct {
	AppID       string
	RedirectURI string
	UserID      meta.ID
	Nonce       string
}

// StartWechatOpenLinkResult 返回给前端的 state 及元数据
type StartWechatOpenLinkResult struct {
	State     string
	Nonce     string
	ExpiresAt time.Time
}

// WechatOpenOAuthStateContext 消费 state 后恢复的上下文。
type WechatOpenOAuthStateContext struct {
	AppID       string
	RedirectURI string
	Nonce       string
	UserID      meta.ID
}

// WechatOpenOAuthStateStarter 创建登录场景 OAuth state challenge
type WechatOpenOAuthStateStarter interface {
	StartWechatOpenLogin(ctx context.Context, input StartWechatOpenLoginInput) (*StartWechatOpenLoginResult, error)
}

// WechatOpenOAuthStateVerifier 校验并一次性消费登录场景 OAuth state
type WechatOpenOAuthStateVerifier interface {
	VerifyAndConsumeWechatOpenLogin(ctx context.Context, state string) (WechatOpenOAuthStateContext, error)
}

// WechatOpenLinkOAuthStateStarter 创建绑定场景 OAuth state challenge
type WechatOpenLinkOAuthStateStarter interface {
	StartWechatOpenLink(ctx context.Context, input StartWechatOpenLinkInput) (*StartWechatOpenLinkResult, error)
}

// WechatOpenLinkOAuthStateVerifier 校验并一次性消费绑定场景 OAuth state
type WechatOpenLinkOAuthStateVerifier interface {
	VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (WechatOpenOAuthStateContext, error)
}

type oauthStateCreator struct {
	repo challengeDomain.Repository
	ttl  time.Duration
	now  func() time.Time
}

type oauthStateVerifier struct {
	domain *challengeDomain.OAuthStateVerifier
	now    func() time.Time
}

func newOAuthStateCreator(repo challengeDomain.Repository, ttl time.Duration) *oauthStateCreator {
	if ttl <= 0 {
		ttl = defaultOAuthStateTTL
	}
	return &oauthStateCreator{
		repo: repo,
		ttl:  ttl,
		now:  time.Now,
	}
}

func newOAuthStateVerifier(repo challengeDomain.Repository) *oauthStateVerifier {
	return &oauthStateVerifier{
		domain: challengeDomain.NewOAuthStateVerifier(repo),
		now:    time.Now,
	}
}

func (c *oauthStateCreator) StartWechatOpenLogin(ctx context.Context, input StartWechatOpenLoginInput) (*StartWechatOpenLoginResult, error) {
	created, err := c.create(ctx, SceneWechatOpenLogin, input.AppID, input.RedirectURI, meta.ZeroID, input.Nonce)
	if err != nil {
		return nil, err
	}
	return &StartWechatOpenLoginResult{State: created.state, Nonce: created.nonce, ExpiresAt: created.expiresAt}, nil
}

func (c *oauthStateCreator) StartWechatOpenLink(ctx context.Context, input StartWechatOpenLinkInput) (*StartWechatOpenLinkResult, error) {
	if input.UserID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "user_id is required for wechat open link state")
	}
	created, err := c.create(ctx, SceneWechatOpenLink, input.AppID, input.RedirectURI, input.UserID, input.Nonce)
	if err != nil {
		return nil, err
	}
	return &StartWechatOpenLinkResult{State: created.state, Nonce: created.nonce, ExpiresAt: created.expiresAt}, nil
}

type createdOAuthState struct {
	state     string
	nonce     string
	expiresAt time.Time
}

func (c *oauthStateCreator) create(ctx context.Context, scene, rawAppID, rawRedirectURI string, userID meta.ID, rawNonce string) (createdOAuthState, error) {
	if c.repo == nil {
		return createdOAuthState{}, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	spec := challengeDomain.OAuthStateSpec{
		Scene:       scene,
		AppID:       strings.TrimSpace(rawAppID),
		RedirectURI: strings.TrimSpace(rawRedirectURI),
		Nonce:       strings.TrimSpace(rawNonce),
		TTL:         c.ttl,
		Now:         c.now(),
	}
	if !userID.IsZero() {
		spec.UserID = userID.String()
	}
	issued, err := challengeDomain.IssueOAuthState(spec)
	if err != nil {
		if isOAuthIssueArgumentError(err) {
			return createdOAuthState{}, perrors.WithCode(code.ErrInvalidArgument, "%s", err.Error())
		}
		return createdOAuthState{}, perrors.WithCode(code.ErrInternalServerError, "%v", err)
	}
	if err := c.repo.Create(ctx, issued.Challenge); err != nil {
		return createdOAuthState{}, err
	}
	return createdOAuthState{state: issued.State, nonce: issued.Nonce, expiresAt: issued.ExpiresAt}, nil
}

func (v *oauthStateVerifier) VerifyAndConsumeWechatOpenLogin(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return v.verifyAndConsume(ctx, SceneWechatOpenLogin, state)
}

func (v *oauthStateVerifier) VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return v.verifyAndConsume(ctx, SceneWechatOpenLink, state)
}

func (v *oauthStateVerifier) verifyAndConsume(ctx context.Context, scene, state string) (WechatOpenOAuthStateContext, error) {
	empty := WechatOpenOAuthStateContext{}
	if v == nil || v.domain == nil {
		return empty, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	verification, err := v.domain.VerifyAndConsume(ctx, challengeDomain.VerifyOAuthStateInput{
		Scene: scene,
		State: state,
		Now:   v.now(),
	})
	if err != nil {
		if isOAuthStateMismatchError(err) || verification.Result.Outcome == challengeDomain.VerificationRejected {
			return empty, perrors.WithCode(code.ErrStateMismatch, "%s", err.Error())
		}
		if verification.Result.Outcome == challengeDomain.VerificationInfrastructureError {
			return empty, perrors.WithCode(code.ErrInternalServerError, "%s", err.Error())
		}
		return empty, err
	}
	if verification.Result.Outcome != challengeDomain.VerificationSuccess {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state already used")
	}
	return wechatOpenOAuthStateContextFromDomain(verification.Context), nil
}

func wechatOpenOAuthStateContextFromDomain(ctx challengeDomain.OAuthStateContext) WechatOpenOAuthStateContext {
	out := WechatOpenOAuthStateContext{
		AppID:       ctx.AppID,
		RedirectURI: ctx.RedirectURI,
		Nonce:       ctx.Nonce,
	}
	if raw := strings.TrimSpace(ctx.UserID); raw != "" {
		if id, err := meta.ParseID(raw); err == nil {
			out.UserID = id
		}
	}
	return out
}

func oauthStateChallengeID(scene, state string) string {
	return challengeDomain.OAuthStateChallengeID(scene, state)
}

func oauthStateSecretHash(state string) []byte {
	return challengeDomain.OAuthStateSecretHash(state)
}

func isOAuthIssueArgumentError(err error) bool {
	return errors.Is(err, challengeDomain.ErrAppIDRequired) ||
		errors.Is(err, challengeDomain.ErrRedirectURIRequired) ||
		errors.Is(err, challengeDomain.ErrOAuthSceneRequired)
}

func isOAuthStateMismatchError(err error) bool {
	return errors.Is(err, challengeDomain.ErrOAuthStateRequired) ||
		errors.Is(err, challengeDomain.ErrOAuthStateNotFound) ||
		errors.Is(err, challengeDomain.ErrOAuthStateInvalid) ||
		errors.Is(err, challengeDomain.ErrOAuthStateAlreadyUsed) ||
		errors.Is(err, challengeDomain.ErrOAuthStateExpired) ||
		errors.Is(err, challengeDomain.ErrOAuthStateMismatch)
}
