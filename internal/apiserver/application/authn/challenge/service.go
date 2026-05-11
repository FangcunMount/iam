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

// Service 短信验证码服务
type Service interface {
	// SendSMSOTP 创建并发送短信验证码
	SendSMSOTP(ctx context.Context, scene, phone string) error
	// VerifyAndConsume 验证并消费短信验证码
	VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) bool
	// DeleteSMSOTP 删除短信验证码
	DeleteSMSOTP(ctx context.Context, scene, phone string) error
}

// service 短信验证码服务
type service struct {
	repo     challengeDomain.Repository
	delivery *SMSOTPDelivery
	creator  Creator
	verifier Verifier
}

// 确保 service 实现了 Service 接口
var _ Service = (*service)(nil)

// NewService 创建短信验证码服务
func NewService(repo challengeDomain.Repository, delivery SMSOTPDelivery, creator Creator, verifier Verifier) Service {
	return &service{repo: repo, delivery: &delivery, creator: creator, verifier: verifier}
}

// SendSMSOTP 创建并发送短信验证码
func (s *service) SendSMSOTP(ctx context.Context, scene, rawPhone string) error {
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
		_ = s.DeleteSMSOTP(ctx, scene, e164)
		return fmt.Errorf("send sms otp: %w", err)
	}
	return nil
}

// VerifyAndConsume 验证并消费短信验证码
func (s *service) VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) bool {
	ok, err := s.verifier.VerifyAndConsume(ctx, scene, rawPhone, otp)
	if err != nil {
		return false
	}
	return ok
}

// DeleteSMSOTP 删除短信验证码
func (s *service) DeleteSMSOTP(ctx context.Context, scene, rawPhone string) error {
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
