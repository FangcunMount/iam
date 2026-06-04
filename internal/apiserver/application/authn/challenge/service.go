package challenge

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

const (
	SceneLoginPhoneOTP   = "login"         // 登录场景
	SceneLinkPhoneOTP    = "link_phone"    // 绑定手机号场景
	defaultSMSOTPTTL     = 5 * time.Minute // 默认短信验证码 TTL
	defaultSMSOTPCodeLen = 6               // 默认短信验证码长度
)

// LoginPhoneOTPSender 发送登录短信验证码。
type LoginPhoneOTPSender interface {
	SendLoginPhoneOTP(ctx context.Context, phone string) error
}

// PhoneLinkOTPSender 发送手机号绑定短信验证码。
type PhoneLinkOTPSender interface {
	SendPhoneLinkOTP(ctx context.Context, phone string) error
}

// LoginPhoneOTPVerifier 验证并消费登录短信验证码。
type LoginPhoneOTPVerifier interface {
	VerifyAndConsumeLoginPhoneOTP(ctx context.Context, phoneE164, otp string) bool
}

// PhoneLinkOTPVerifier 验证并消费手机号绑定短信验证码。
type PhoneLinkOTPVerifier interface {
	VerifyAndConsumePhoneLinkOTP(ctx context.Context, phoneE164, otp string) bool
}

// Service 是 challenge 包对外暴露的验证码与 OAuth state 用例集合。
type Service interface {
	LoginPhoneOTPSender
	PhoneLinkOTPSender
	LoginPhoneOTPVerifier
	PhoneLinkOTPVerifier
	WechatOpenOAuthStateStarter
	WechatOpenOAuthStateVerifier
	WechatOpenLinkOAuthStateStarter
	WechatOpenLinkOAuthStateVerifier
}

// service 短信验证码与 OAuth state 服务
type service struct {
	repo          challengeDomain.Repository
	delivery      *SMSOTPDelivery
	creator       Creator
	verifier      Verifier
	oauthCreator  *oauthStateCreator
	oauthVerifier *oauthStateVerifier
}

// 确保 service 实现了 Service 接口
var _ Service = (*service)(nil)

// NewService 创建 challenge 应用服务。
func NewService(repo challengeDomain.Repository, delivery SMSOTPDelivery, creator Creator, verifier Verifier) Service {
	return &service{
		repo:          repo,
		delivery:      &delivery,
		creator:       creator,
		verifier:      verifier,
		oauthCreator:  newOAuthStateCreator(repo, defaultOAuthStateTTL),
		oauthVerifier: newOAuthStateVerifier(repo),
	}
}

func (s *service) StartWechatOpenLogin(ctx context.Context, input StartWechatOpenLoginInput) (*StartWechatOpenLoginResult, error) {
	return s.oauthCreator.StartWechatOpenLogin(ctx, input)
}

func (s *service) VerifyAndConsumeWechatOpenLogin(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return s.oauthVerifier.VerifyAndConsumeWechatOpenLogin(ctx, state)
}

func (s *service) StartWechatOpenLink(ctx context.Context, input StartWechatOpenLinkInput) (*StartWechatOpenLinkResult, error) {
	return s.oauthCreator.StartWechatOpenLink(ctx, input)
}

func (s *service) VerifyAndConsumeWechatOpenLink(ctx context.Context, state string) (WechatOpenOAuthStateContext, error) {
	return s.oauthVerifier.VerifyAndConsumeWechatOpenLink(ctx, state)
}

// SendLoginPhoneOTP 创建并发送登录短信验证码。
func (s *service) SendLoginPhoneOTP(ctx context.Context, rawPhone string) error {
	return s.sendSMSOTP(ctx, SceneLoginPhoneOTP, rawPhone)
}

// SendPhoneLinkOTP 创建并发送手机号绑定短信验证码。
func (s *service) SendPhoneLinkOTP(ctx context.Context, rawPhone string) error {
	return s.sendSMSOTP(ctx, SceneLinkPhoneOTP, rawPhone)
}

// sendSMSOTP 创建并发送短信验证码。
func (s *service) sendSMSOTP(ctx context.Context, scene, rawPhone string) error {
	if s.delivery == nil || s.delivery.Gate == nil || s.delivery.SMS == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "sms otp delivery is not configured")
	}
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "challenge scene is required")
	}
	e164 := phone.String()
	// L1 冷却：同号同场景在冷却窗口内不可重复发送（失败不回退，防重试轰炸）。
	ok, err := s.delivery.Gate.TryAcquire(ctx, e164, scene, s.delivery.effectiveCooldown())
	if err != nil {
		return fmt.Errorf("sms otp send gate: %w", err)
	}
	if !ok {
		return perrors.WithCode(code.ErrOTPSendTooFrequent, "please wait before requesting another code")
	}

	// L2/L3 限量：小时/天固定窗口计数；发送失败时回退计数。
	rollbackQuota, ok, err := s.acquireSendQuota(ctx, e164, scene)
	if err != nil {
		return fmt.Errorf("sms otp send quota: %w", err)
	}
	if !ok {
		return perrors.WithCode(code.ErrOTPSendTooFrequent, "sms quota exceeded, please try again later")
	}

	// 创建短信验证码
	challenge, err := s.creator.CreateSMSOTP(
		ctx,
		scene,
		e164,
		WithTTL(s.delivery.effectiveTTL()),
		WithCodeLen(s.delivery.effectiveCodeLen()),
	)
	if err != nil {
		rollbackQuota()
		return err
	}
	if err := s.delivery.SMS.SendLoginOTP(ctx, e164, challenge.Code); err != nil {
		rollbackQuota()
		_ = s.deleteSMSOTP(ctx, scene, e164)
		return fmt.Errorf("send sms otp: %w", err)
	}
	return nil
}

const (
	quotaDimensionHourly = "hourly"
	quotaDimensionDaily  = "daily"
	quotaWindowHourly    = time.Hour
	quotaWindowDaily     = 24 * time.Hour
)

// acquireSendQuota 按小时/天维度累计发送次数，任一维度超限即拒绝。
// 返回的 rollback 用于在后续步骤失败时回退已累计的计数（无副作用、可安全多次调用）。
func (s *service) acquireSendQuota(ctx context.Context, e164, scene string) (rollback func(), ok bool, err error) {
	noop := func() {}
	if s.delivery == nil || s.delivery.Quota == nil {
		return noop, true, nil
	}

	type consumed struct {
		dimension string
		window    time.Duration
	}
	var acquired []consumed
	rollback = func() {
		for _, c := range acquired {
			_ = s.delivery.Quota.Rollback(ctx, e164, scene, c.dimension, c.window)
		}
		acquired = nil
	}

	dims := []consumed{
		{quotaDimensionHourly, quotaWindowHourly},
		{quotaDimensionDaily, quotaWindowDaily},
	}
	limits := map[string]int{
		quotaDimensionHourly: s.delivery.effectiveHourlyLimit(),
		quotaDimensionDaily:  s.delivery.effectiveDailyLimit(),
	}
	for _, d := range dims {
		limit := limits[d.dimension]
		if limit <= 0 {
			continue
		}
		allowed, err := s.delivery.Quota.TryConsume(ctx, e164, scene, d.dimension, limit, d.window)
		if err != nil {
			rollback()
			return noop, false, err
		}
		if !allowed {
			rollback()
			return noop, false, nil
		}
		acquired = append(acquired, d)
	}
	return rollback, true, nil
}

// VerifyAndConsumeLoginPhoneOTP 验证并消费登录短信验证码。
func (s *service) VerifyAndConsumeLoginPhoneOTP(ctx context.Context, rawPhone, otp string) bool {
	return s.verifyAndConsume(ctx, SceneLoginPhoneOTP, rawPhone, otp)
}

// VerifyAndConsumePhoneLinkOTP 验证并消费手机号绑定短信验证码。
func (s *service) VerifyAndConsumePhoneLinkOTP(ctx context.Context, rawPhone, otp string) bool {
	return s.verifyAndConsume(ctx, SceneLinkPhoneOTP, rawPhone, otp)
}

// verifyAndConsume 验证并消费短信验证码。
func (s *service) verifyAndConsume(ctx context.Context, scene, rawPhone, otp string) bool {
	ok, err := s.verifier.VerifyAndConsume(ctx, scene, rawPhone, otp)
	if err != nil {
		return false
	}
	return ok
}

// deleteSMSOTP 删除短信验证码。
func (s *service) deleteSMSOTP(ctx context.Context, scene, rawPhone string) error {
	if s.repo == nil {
		return perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	return s.repo.Delete(ctx, smsOTPChallengeID(strings.TrimSpace(scene), phone.String()))
}

// smsOTPChallengeID 生成短信验证码挑战 ID
func smsOTPChallengeID(scene, phoneE164 string) string {
	return fmt.Sprintf("sms_otp:%s:%s", strings.TrimSpace(scene), strings.TrimSpace(phoneE164))
}

// smsOTPSecretHash 生成短信验证码挑战密钥
func smsOTPSecretHash(scene, phoneE164, otp string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(scene) + "\x00" + strings.TrimSpace(phoneE164) + "\x00" + strings.TrimSpace(otp)))
	return sum[:]
}
