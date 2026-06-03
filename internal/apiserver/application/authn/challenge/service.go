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
	ok, err := s.delivery.Gate.TryAcquire(ctx, e164, scene, s.delivery.effectiveCooldown())
	if err != nil {
		return fmt.Errorf("sms otp send gate: %w", err)
	}
	if !ok {
		return perrors.WithCode(code.ErrOTPSendTooFrequent, "please wait before requesting another code")
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
		return err
	}
	if err := s.delivery.SMS.SendLoginOTP(ctx, e164, challenge.Code); err != nil {
		_ = s.deleteSMSOTP(ctx, scene, e164)
		return fmt.Errorf("send sms otp: %w", err)
	}
	return nil
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
