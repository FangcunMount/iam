package challenge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

const (
	// SceneWechatOpenLogin 微信开放平台扫码登录场景（user_id 必空）。
	SceneWechatOpenLogin = "wechat_open_login"
	// SceneWechatOpenLink 微信开放平台扫码绑定场景（user_id 必填，来自已登录用户）。
	SceneWechatOpenLink = "wechat_open_link"

	PayloadKeyAppID       = "app_id"
	PayloadKeyRedirectURI = "redirect_uri"
	PayloadKeyNonce       = "nonce"
	PayloadKeyUserID      = "user_id"

	defaultOAuthStateTTL  = 10 * time.Minute
	oauthStateRandomBytes = 32
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
//
// UserID 来自已登录的 Operating 用户，由 callback usecase 注入，禁止来自前端 payload。
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
//
// UserID 仅在绑定场景非空；登录场景必为零值。
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

// WechatOpenOAuthStateCreator 创建微信开放平台扫码登录 OAuth state
type oauthStateCreator struct {
	repo challengeDomain.Repository
	ttl  time.Duration
	now  func() time.Time
}

// WechatOpenOAuthStateVerifier 校验并一次性消费微信开放平台扫码登录 OAuth state
type oauthStateVerifier struct {
	repo challengeDomain.Repository
	now  func() time.Time
}

// newOAuthStateCreator 创建微信开放平台扫码登录 OAuth state 创建器
func newOAuthStateCreator(repo challengeDomain.Repository, ttl time.Duration) *oauthStateCreator {
	if ttl <= 0 {
		ttl = defaultOAuthStateTTL
	}
	return &oauthStateCreator{repo: repo, ttl: ttl, now: time.Now}
}

// newOAuthStateVerifier 创建微信开放平台扫码登录 OAuth state 验证器
func newOAuthStateVerifier(repo challengeDomain.Repository) *oauthStateVerifier {
	return &oauthStateVerifier{repo: repo, now: time.Now}
}

// StartWechatOpenLogin 创建微信开放平台扫码登录 OAuth state（user_id 必空）
func (c *oauthStateCreator) StartWechatOpenLogin(ctx context.Context, input StartWechatOpenLoginInput) (*StartWechatOpenLoginResult, error) {
	created, err := c.create(ctx, SceneWechatOpenLogin, input.AppID, input.RedirectURI, meta.ZeroID, input.Nonce)
	if err != nil {
		return nil, err
	}
	return &StartWechatOpenLoginResult{State: created.state, Nonce: created.nonce, ExpiresAt: created.expiresAt}, nil
}

// StartWechatOpenLink 创建微信开放平台扫码绑定 OAuth state（user_id 必填）
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

// createdOAuthState 内部创建结果。
type createdOAuthState struct {
	state     string
	nonce     string
	expiresAt time.Time
}

// create 创建指定场景的 OAuth state challenge。userID 为零值时不写入 payload。
func (c *oauthStateCreator) create(ctx context.Context, scene, rawAppID, rawRedirectURI string, userID meta.ID, rawNonce string) (createdOAuthState, error) {
	if c.repo == nil {
		return createdOAuthState{}, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	appID := strings.TrimSpace(rawAppID)
	redirectURI := strings.TrimSpace(rawRedirectURI)
	if appID == "" {
		return createdOAuthState{}, perrors.WithCode(code.ErrInvalidArgument, "app_id is required")
	}
	if redirectURI == "" {
		return createdOAuthState{}, perrors.WithCode(code.ErrInvalidArgument, "redirect_uri is required")
	}

	nonce := strings.TrimSpace(rawNonce)
	if nonce == "" {
		var err error
		nonce, err = randomOAuthToken()
		if err != nil {
			return createdOAuthState{}, perrors.WithCode(code.ErrInternalServerError, "failed to generate oauth nonce: %v", err)
		}
	}

	state, err := randomOAuthToken()
	if err != nil {
		return createdOAuthState{}, perrors.WithCode(code.ErrInternalServerError, "failed to generate oauth state: %v", err)
	}

	payload := map[string]string{
		PayloadKeyAppID:       appID,
		PayloadKeyRedirectURI: redirectURI,
		PayloadKeyNonce:       nonce,
	}
	if !userID.IsZero() {
		payload[PayloadKeyUserID] = userID.String()
	}

	now := c.now()
	expiresAt := now.Add(c.ttl)
	challenge := &challengeDomain.AuthChallenge{
		ID:         oauthStateChallengeID(scene, state),
		Type:       challengeDomain.TypeOAuthState,
		Scene:      scene,
		Target:     appID,
		SecretHash: oauthStateSecretHash(state),
		Payload:    payload,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
	}
	if err := c.repo.Create(ctx, challenge); err != nil {
		return createdOAuthState{}, err
	}
	return createdOAuthState{state: state, nonce: nonce, expiresAt: expiresAt}, nil
}

// VerifyAndConsumeWechatOpenLogin 校验并一次性消费登录场景 OAuth state
func (v *oauthStateVerifier) VerifyAndConsumeWechatOpenLogin(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return v.verifyAndConsume(ctx, SceneWechatOpenLogin, state)
}

// VerifyAndConsumeWechatOpenLink 校验并一次性消费绑定场景 OAuth state
func (v *oauthStateVerifier) VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return v.verifyAndConsume(ctx, SceneWechatOpenLink, state)
}

// verifyAndConsume 按场景校验并一次性消费 OAuth state。
func (v *oauthStateVerifier) verifyAndConsume(ctx context.Context, scene, state string) (WechatOpenOAuthStateContext, error) {
	empty := WechatOpenOAuthStateContext{}
	if v.repo == nil {
		return empty, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state is required")
	}

	challengeID := oauthStateChallengeID(scene, state)
	challenge, err := v.repo.Get(ctx, challengeID)
	if err != nil {
		return empty, err
	}
	if challenge == nil {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state not found")
	}
	if challenge.Type != challengeDomain.TypeOAuthState || challenge.Scene != scene {
		return empty, perrors.WithCode(code.ErrStateMismatch, "invalid oauth state challenge")
	}
	now := v.now()
	if challenge.IsConsumed() {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state already used")
	}
	if challenge.IsExpired(now) {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state expired")
	}
	if subtle.ConstantTimeCompare(challenge.SecretHash, oauthStateSecretHash(state)) != 1 {
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state mismatch")
	}
	consumed, err := v.repo.ConsumeIfSecretMatches(ctx, challengeID, challenge.SecretHash)
	if err != nil {
		return empty, err
	}
	if !consumed {
		logger.L(ctx).Infow("oauth state consumption rejected because it was already consumed",
			"challenge_type", challengeDomain.TypeOAuthState,
			"scene", scene,
		)
		return empty, perrors.WithCode(code.ErrStateMismatch, "oauth state already used")
	}
	return wechatOpenOAuthStateContextFromPayload(challenge.Payload), nil
}

// wechatOpenOAuthStateContextFromPayload 从 OAuth state 的 payload 中恢复上下文
func wechatOpenOAuthStateContextFromPayload(payload map[string]string) WechatOpenOAuthStateContext {
	if payload == nil {
		return WechatOpenOAuthStateContext{}
	}
	out := WechatOpenOAuthStateContext{
		AppID:       payload[PayloadKeyAppID],
		RedirectURI: payload[PayloadKeyRedirectURI],
		Nonce:       payload[PayloadKeyNonce],
	}
	if raw := strings.TrimSpace(payload[PayloadKeyUserID]); raw != "" {
		if id, err := meta.ParseID(raw); err == nil {
			out.UserID = id
		}
	}
	return out
}

// oauthStateChallengeID 构造 OAuth state 挑战 ID
func oauthStateChallengeID(scene, state string) string {
	return fmt.Sprintf("oauth_state:%s:%s", strings.TrimSpace(scene), strings.TrimSpace(state))
}

// oauthStateSecretHash 计算 OAuth state 的密钥哈希
func oauthStateSecretHash(state string) []byte {
	sum := sha256.Sum256([]byte("oauth_state\x00" + strings.TrimSpace(state)))
	return sum[:]
}

// randomOAuthToken 生成随机 OAuth token
func randomOAuthToken() (string, error) {
	buf := make([]byte, oauthStateRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
