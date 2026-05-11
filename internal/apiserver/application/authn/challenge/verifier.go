package challenge

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Verifier 短信验证码验证器
type Verifier interface {
	VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) (bool, error)
}

// Verifier 短信验证码验证器
type verifier struct {
	repo challengeDomain.Repository
}

// 确保 verifier 实现 Verifier 接口
var _ Verifier = (*verifier)(nil)

// NewVerifier 创建短信验证码验证器
func NewVerifier(repo challengeDomain.Repository) Verifier {
	return &verifier{repo: repo}
}

// VerifyAndConsume 验证并消费短信验证码
func (s *verifier) VerifyAndConsume(ctx context.Context, scene, rawPhone, otp string) (bool, error) {
	if s.repo == nil {
		return false, perrors.WithCode(code.ErrInternalServerError, "challenge repository is not configured")
	}
	// 校验手机号格式
	phone, err := meta.NewPhone(rawPhone)
	if err != nil {
		return false, perrors.WithCode(code.ErrInvalidArgument, "invalid phone: %v", err)
	}
	// 校验场景
	scene = strings.TrimSpace(scene)
	otp = strings.TrimSpace(otp)
	if scene == "" || otp == "" {
		return false, nil
	}
	e164 := phone.String()
	// 根据 scene + phone 构造 challengeID
	challengeID := smsOTPChallengeID(scene, e164)
	// 从 Repository 获取 Challenge
	challenge, err := s.repo.Get(ctx, challengeID)
	if err != nil {
		return false, err
	}
	// 检查 Challenge 是否存在
	if challenge == nil || challenge.Type != challengeDomain.TypeSMSOTP || challenge.IsConsumed() || challenge.IsExpired(time.Now()) {
		return false, nil
	}
	// 计算请求 OTP 的 SecretHash
	expected := smsOTPSecretHash(scene, e164, otp)
	if subtle.ConstantTimeCompare(challenge.SecretHash, expected) != 1 {
		return false, nil
	}
	// 校验成功后调用 Repository.Consume
	if err := s.repo.Consume(ctx, challengeID); err != nil {
		return false, err
	}
	// 返回 true
	return true, nil
}
