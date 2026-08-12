package challenge

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

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

// Verifier 短信验证码验证器
type verifier struct {
	repo        challengeDomain.Repository
	maxAttempts int
}

// 确保 verifier 实现 Verifier 接口
var _ Verifier = (*verifier)(nil)

// NewVerifier 创建短信验证码验证器
func NewVerifier(repo challengeDomain.Repository, configuredMaxAttempts ...int) Verifier {
	maxAttempts := 5
	if len(configuredMaxAttempts) > 0 && configuredMaxAttempts[0] > 0 {
		maxAttempts = configuredMaxAttempts[0]
	}
	return &verifier{repo: repo, maxAttempts: maxAttempts}
}

// VerifyAndConsume 验证并消费短信验证码
func (s *verifier) VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) (bool, error) {
	if s.repo == nil {
		recordOTPVerification(scene, "failed")
		return false, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	// 校验手机号格式
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		recordOTPVerification(scene, "invalid")
		return false, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	// 校验场景
	scene = strings.TrimSpace(scene)
	otp = strings.TrimSpace(otp)
	if scene == "" || otp == "" {
		recordOTPVerification(scene, "invalid")
		return false, nil
	}
	e164 := phone.String()
	// 根据 scene + phone 构造 challengeID
	challengeID := smsOTPChallengeID(scene, e164)
	// 从 Repository 获取 Challenge
	challenge, err := s.repo.Get(ctx, challengeID)
	if err != nil {
		recordOTPVerification(scene, "failed")
		return false, err
	}
	// 检查 Challenge 是否存在
	if challenge == nil || challenge.Type != challengeDomain.TypeSMSOTP || challenge.IsConsumed() || challenge.IsExpired(time.Now()) {
		recordOTPVerification(scene, "rejected")
		return false, nil
	}
	// 计算请求 OTP 的 SecretHash
	expected := smsOTPSecretHash(scene, e164, otp)
	if subtle.ConstantTimeCompare(challenge.SecretHash, expected) != 1 {
		current, exhausted, recordErr := s.repo.RecordFailedAttemptIfCurrent(
			ctx,
			challengeID,
			challenge.SecretHash,
			s.maxAttempts,
		)
		if recordErr != nil {
			recordOTPVerification(scene, "failed")
			return false, recordErr
		}
		if current && exhausted {
			recordOTPVerification(scene, "exhausted")
			logger.L(ctx).Infow("challenge rejected after maximum verification attempts",
				"challenge_type", challengeDomain.TypeSMSOTP,
				"scene", scene,
			)
		}
		if !exhausted {
			recordOTPVerification(scene, "rejected")
		}
		return false, nil
	}
	// 校验成功后调用 Repository.Consume
	consumed, err := s.repo.ConsumeIfSecretMatches(ctx, challengeID, expected)
	if err != nil {
		recordOTPVerification(scene, "failed")
		return false, err
	}
	if !consumed {
		recordOTPVerification(scene, "rejected")
		logger.L(ctx).Infow("challenge consumption rejected because it was replaced or already consumed",
			"challenge_type", challengeDomain.TypeSMSOTP,
			"scene", scene,
		)
		return false, nil
	}
	recordOTPVerification(scene, "success")
	// 返回 true
	return true, nil
}
