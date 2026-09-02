package challenge

import (
	"context"
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
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	options := normalizeSMSOTPOptions(opts...)
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "challenge scene is required")
	}

	issued, err := challengeDomain.IssueSMSOTP(challengeDomain.SMSOTPSpec{
		Scene:     scene,
		PhoneE164: phone.String(),
		TTL:       options.ttl,
		CodeLen:   options.codeLen,
		Now:       options.now,
	})
	if err != nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "failed to issue sms otp challenge: %v", err)
	}
	if err := s.repo.Create(ctx, issued.Challenge); err != nil {
		return nil, err
	}
	return &SMSOTP{
		Scene:     scene,
		PhoneE164: phone.String(),
		Code:      issued.PlainOTP,
		ExpiresAt: issued.Challenge.ExpiresAt,
	}, nil
}

func normalizeSMSOTPOptions(opts ...SMSOTPOption) smsOTPOptions {
	o := smsOTPOptions{
		ttl:     challengeDomain.DefaultSMSOTPTTL,
		codeLen: challengeDomain.DefaultSMSOTPCodeLen,
		now:     time.Now(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.ttl <= 0 {
		o.ttl = challengeDomain.DefaultSMSOTPTTL
	}
	if o.codeLen <= 0 {
		o.codeLen = challengeDomain.DefaultSMSOTPCodeLen
	}
	if o.codeLen > challengeDomain.MaxSMSOTPCodeLen {
		o.codeLen = challengeDomain.MaxSMSOTPCodeLen
	}
	if o.now.IsZero() {
		o.now = time.Now()
	}
	return o
}
