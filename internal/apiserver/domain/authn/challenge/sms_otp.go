package challenge

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// SMSOTP 是短信验证码挑战方式。
type SMSOTP struct{}

// SMSOTPSpec 短信验证码挑战规格。
type SMSOTPSpec struct {
	Scene     string
	PhoneE164 string
	OTP       string
	TTL       time.Duration
	CodeLen   int
	Now       time.Time
}

// SMSOTPIssueResult 短信验证码签发结果。
type SMSOTPIssueResult struct {
	Challenge *AuthChallenge
	PlainOTP  string
}

// Issue 创建短信验证码挑战实体。
func (SMSOTP) Issue(spec SMSOTPSpec) (*SMSOTPIssueResult, error) {
	scene := strings.TrimSpace(spec.Scene)
	if scene == "" {
		return nil, ErrChallengeSceneRequired
	}
	phoneE164 := strings.TrimSpace(spec.PhoneE164)
	if phoneE164 == "" {
		return nil, ErrPhoneE164Required
	}

	ttl := spec.TTL
	if ttl <= 0 {
		ttl = DefaultSMSOTPTTL
	}
	codeLen := spec.CodeLen
	if codeLen <= 0 {
		codeLen = DefaultSMSOTPCodeLen
	}
	if codeLen > MaxSMSOTPCodeLen {
		codeLen = MaxSMSOTPCodeLen
	}
	now := normalizeVerificationTime(spec.Now)

	otp := strings.TrimSpace(spec.OTP)
	if otp == "" {
		generated, err := randomNumericOTP(codeLen)
		if err != nil {
			return nil, fmt.Errorf("generate otp: %w", err)
		}
		otp = generated
	}

	expiresAt := now.Add(ttl)
	challenge := &AuthChallenge{
		ID:         SMSOTPChallengeID(scene, phoneE164),
		Type:       TypeSMSOTP,
		Scene:      scene,
		Target:     phoneE164,
		SecretHash: SMSOTPSecretHash(scene, phoneE164, otp),
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
	}
	return &SMSOTPIssueResult{Challenge: challenge, PlainOTP: otp}, nil
}

// IssueSMSOTP 创建短信验证码挑战实体。
func IssueSMSOTP(spec SMSOTPSpec) (*SMSOTPIssueResult, error) {
	return SMSOTP{}.Issue(spec)
}

// VerifySMSOTPInput 短信验证码校验输入。
type VerifySMSOTPInput struct {
	Scene     string
	PhoneE164 string
	OTP       string
	Now       time.Time
}

// SMSOTPVerifier 短信验证码校验器。
type SMSOTPVerifier struct {
	Repo        Repository
	MaxAttempts int
}

// NewSMSOTPVerifier 创建短信验证码校验器。
func NewSMSOTPVerifier(repo Repository, configuredMaxAttempts ...int) *SMSOTPVerifier {
	if repo == nil {
		return nil
	}
	maxAttempts := DefaultMaxVerifyAttempts
	if len(configuredMaxAttempts) > 0 && configuredMaxAttempts[0] > 0 {
		maxAttempts = configuredMaxAttempts[0]
	}
	return &SMSOTPVerifier{Repo: repo, MaxAttempts: maxAttempts}
}

// VerifyAndConsume 校验并消费短信验证码挑战。
func (v *SMSOTPVerifier) VerifyAndConsume(ctx context.Context, input VerifySMSOTPInput) (VerificationResult, error) {
	if v == nil || v.Repo == nil {
		return VerificationResult{Outcome: VerificationInfrastructureError}, ErrRepositoryNotConfigured
	}
	scene := strings.TrimSpace(input.Scene)
	otp := strings.TrimSpace(input.OTP)
	phoneE164 := strings.TrimSpace(input.PhoneE164)
	if scene == "" || otp == "" || phoneE164 == "" {
		return VerificationResult{Outcome: VerificationInvalidInput}, nil
	}
	now := normalizeVerificationTime(input.Now)

	challengeID := SMSOTPChallengeID(scene, phoneE164)
	challenge, err := v.Repo.Get(ctx, challengeID)
	if err != nil {
		return VerificationResult{Outcome: VerificationInfrastructureError}, err
	}
	if AssessUsability(challenge, now, TypeSMSOTP, scene) != UsabilityOK {
		return VerificationResult{Outcome: VerificationRejected}, nil
	}

	expected := SMSOTPSecretHash(scene, phoneE164, otp)
	if !secretHashMatches(challenge.SecretHash, expected) {
		return recordFailedVerification(ctx, v.Repo, challengeID, challenge.SecretHash, v.MaxAttempts)
	}
	return consumeOnce(ctx, v.Repo, challengeID, expected)
}

func randomNumericOTP(length int) (string, error) {
	if length <= 0 || length > MaxSMSOTPCodeLen {
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
