package challenge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"math/big"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

const (
	SceneLoginPhoneOTP = "login"
	SceneLinkPhoneOTP  = "link_phone"

	defaultSMSOTPTTL     = 5 * time.Minute
	defaultSMSOTPCodeLen = 6
)

// Service 手机验证码服务
type Service interface {
	SendSMSOTP(ctx context.Context, scene, phone string) error
	CreateSMSOTP(ctx context.Context, scene, phone string, opts ...SMSOTPOption) (*SMSOTP, error)
	VerifyAndConsumeSMSOTP(ctx context.Context, scene, phone, code string) (bool, error)
	VerifyAndConsume(ctx context.Context, phoneE164, scene, code string) bool
	DeleteSMSOTP(ctx context.Context, scene, phone string) error
}

// SMSOTPDelivery 是短信验证码发送依赖。
type SMSOTPDelivery struct {
	Gate     authentication.OTPSendGate
	SMS      authentication.SMSSender
	TTL      time.Duration
	Cooldown time.Duration
	CodeLen  int
}

// SMSOTP 手机验证码
type SMSOTP struct {
	Scene     string
	PhoneE164 string
	Code      string
	ExpiresAt time.Time
}

// SMSOTPOption 手机验证码选项
type SMSOTPOption func(*smsOTPOptions)

// smsOTPOptions 手机验证码选项
type smsOTPOptions struct {
	ttl     time.Duration
	codeLen int
	now     time.Time
}

type Option func(*service)

// WithSMSOTPDelivery 配置短信验证码发送依赖。
func WithSMSOTPDelivery(delivery SMSOTPDelivery) Option {
	return func(s *service) {
		s.delivery = &delivery
	}
}

// WithTTL 设置验证码有效期
func WithTTL(ttl time.Duration) SMSOTPOption {
	return func(o *smsOTPOptions) { o.ttl = ttl }
}

// WithCodeLen 设置验证码长度
func WithCodeLen(codeLen int) SMSOTPOption {
	return func(o *smsOTPOptions) { o.codeLen = codeLen }
}

// WithNow 设置当前时间
func WithNow(now time.Time) SMSOTPOption {
	return func(o *smsOTPOptions) { o.now = now }
}

// service 手机验证码服务
type service struct {
	repo     challengeDomain.Repository
	delivery *SMSOTPDelivery
}

// NewService 创建手机验证码服务
func NewService(repo challengeDomain.Repository, opts ...Option) Service {
	s := &service{repo: repo}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// SendSMSOTP 创建并发送手机验证码。
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

	challenge, err := s.CreateSMSOTP(
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

// CreateSMSOTP 创建手机验证码
func (s *service) CreateSMSOTP(ctx context.Context, scene, rawPhone string, opts ...SMSOTPOption) (*SMSOTP, error) {
	if s.repo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	options := normalizeSMSOTPOptions(opts...)
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "challenge scene is required")
	}
	otp, err := randomNumericOTP(options.codeLen)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "failed to generate otp: %v", err)
	}

	e164 := phone.String()
	expiresAt := options.now.Add(options.ttl)
	challenge := &challengeDomain.AuthChallenge{
		ID:         smsOTPChallengeID(scene, e164),
		Type:       challengeDomain.TypeSMSOTP,
		Scene:      scene,
		Target:     e164,
		SecretHash: smsOTPSecretHash(scene, e164, otp),
		ExpiresAt:  expiresAt,
		CreatedAt:  options.now,
	}
	if err := s.repo.Create(ctx, challenge); err != nil {
		return nil, err
	}
	return &SMSOTP{
		Scene:     scene,
		PhoneE164: e164,
		Code:      otp,
		ExpiresAt: expiresAt,
	}, nil
}

// VerifyAndConsumeSMSOTP 验证并消费手机验证码
func (s *service) VerifyAndConsumeSMSOTP(ctx context.Context, scene, rawPhone, otp string) (bool, error) {
	if s.repo == nil {
		return false, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return false, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	scene = strings.TrimSpace(scene)
	otp = strings.TrimSpace(otp)
	if scene == "" || otp == "" {
		return false, nil
	}
	e164 := phone.String()
	challengeID := smsOTPChallengeID(scene, e164)
	challenge, err := s.repo.Get(ctx, challengeID)
	if err != nil {
		return false, err
	}
	if challenge == nil || challenge.Type != challengeDomain.TypeSMSOTP || challenge.IsConsumed() || challenge.IsExpired(time.Now()) {
		return false, nil
	}
	expected := smsOTPSecretHash(scene, e164, otp)
	if subtle.ConstantTimeCompare(challenge.SecretHash, expected) != 1 {
		return false, nil
	}
	if err := s.repo.Consume(ctx, challengeID); err != nil {
		return false, err
	}
	return true, nil
}

// VerifyAndConsume 验证并消费手机验证码
func (s *service) VerifyAndConsume(ctx context.Context, phoneE164, scene, otp string) bool {
	ok, err := s.VerifyAndConsumeSMSOTP(ctx, scene, phoneE164, otp)
	return err == nil && ok
}

// DeleteSMSOTP 删除手机验证码
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

// normalizeSMSOTPOptions 规范化手机验证码选项
func normalizeSMSOTPOptions(opts ...SMSOTPOption) smsOTPOptions {
	o := smsOTPOptions{
		ttl:     defaultSMSOTPTTL,
		codeLen: defaultSMSOTPCodeLen,
		now:     time.Now(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.ttl <= 0 {
		o.ttl = defaultSMSOTPTTL
	}
	if o.codeLen <= 0 {
		o.codeLen = defaultSMSOTPCodeLen
	}
	if o.codeLen > 12 {
		o.codeLen = 12
	}
	if o.now.IsZero() {
		o.now = time.Now()
	}
	return o
}

func (d *SMSOTPDelivery) effectiveTTL() time.Duration {
	if d == nil || d.TTL <= 0 {
		return defaultSMSOTPTTL
	}
	return d.TTL
}

func (d *SMSOTPDelivery) effectiveCooldown() time.Duration {
	if d == nil || d.Cooldown <= 0 {
		return 60 * time.Second
	}
	return d.Cooldown
}

func (d *SMSOTPDelivery) effectiveCodeLen() int {
	if d == nil || d.CodeLen <= 0 {
		return defaultSMSOTPCodeLen
	}
	if d.CodeLen > 12 {
		return 12
	}
	return d.CodeLen
}

func smsOTPChallengeID(scene, phoneE164 string) string {
	return fmt.Sprintf("sms_otp:%s:%s", strings.TrimSpace(scene), strings.TrimSpace(phoneE164))
}

func smsOTPSecretHash(scene, phoneE164, otp string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(scene) + "\x00" + strings.TrimSpace(phoneE164) + "\x00" + strings.TrimSpace(otp)))
	return sum[:]
}

func randomNumericOTP(length int) (string, error) {
	if length <= 0 || length > 12 {
		return "", fmt.Errorf("invalid otp length %d", length)
	}
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("rand otp digit: %w", err)
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}
