package challenge

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Verifier 短信验证码验证器
type Verifier interface {
	VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) (bool, error)
}

type verifier struct {
	domain *challengeDomain.SMSOTPVerifier
}

var _ Verifier = (*verifier)(nil)

// NewVerifier 创建短信验证码验证器
func NewVerifier(repo challengeDomain.Repository, configuredMaxAttempts ...int) Verifier {
	return &verifier{domain: challengeDomain.NewSMSOTPVerifier(repo, configuredMaxAttempts...)}
}

// VerifyAndConsume 验证并消费短信验证码
func (s *verifier) VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) (bool, error) {
	if s.domain == nil {
		recordOTPVerification(scene, "failed")
		return false, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		recordOTPVerification(scene, "invalid")
		return false, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	scene = strings.TrimSpace(scene)
	otp = strings.TrimSpace(otp)
	if scene == "" || otp == "" {
		recordOTPVerification(scene, "invalid")
		return false, nil
	}

	result, err := s.domain.VerifyAndConsume(ctx, challengeDomain.VerifySMSOTPInput{
		Scene:     scene,
		PhoneE164: phone.String(),
		OTP:       otp,
	})
	if err != nil {
		recordOTPVerification(scene, "failed")
		return false, err
	}
	switch result.Outcome {
	case challengeDomain.VerificationSuccess:
		recordOTPVerification(scene, "success")
		return true, nil
	case challengeDomain.VerificationExhausted:
		recordOTPVerification(scene, "exhausted")
		logger.L(ctx).Infow("challenge rejected after maximum verification attempts",
			"challenge_type", challengeDomain.TypeSMSOTP,
			"scene", scene,
		)
		return false, nil
	case challengeDomain.VerificationRejected:
		recordOTPVerification(scene, "rejected")
		return false, nil
	case challengeDomain.VerificationInvalidInput:
		recordOTPVerification(scene, "invalid")
		return false, nil
	default:
		recordOTPVerification(scene, "failed")
		return false, nil
	}
}
