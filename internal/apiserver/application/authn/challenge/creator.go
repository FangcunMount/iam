package challenge

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Creator 短信验证码创建器
type Creator interface {
	CreateSMSOTP(ctx context.Context, scene, phone string, opts ...SMSOTPOption) (*SMSOTP, error)
}

// Creator 短信验证码创建器
type creator struct {
	repo challengeDomain.Repository
}

// NewCreator 创建短信验证码创建器
func NewCreator(repo challengeDomain.Repository) Creator {
	return &creator{repo: repo}
}

// CreateSMSOTP 创建短信验证码
func (s *creator) CreateSMSOTP(ctx context.Context, scene, rawPhone string, opts ...SMSOTPOption) (*SMSOTP, error) {
	if s.repo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	// 校验手机号格式
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	// 规范化短信验证码选项
	options := normalizeSMSOTPOptions(opts...)
	// 校验场景
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "challenge scene is required")
	}
	// 生成随机数字验证码
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

// normalizeSMSOTPOptions 规范化短信验证码选项
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

// randomNumericOTP 生成随机数字验证码
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
