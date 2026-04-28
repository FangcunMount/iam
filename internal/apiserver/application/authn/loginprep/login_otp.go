package loginprep

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
)

// PhoneOTPDeps 手机登录发码依赖（与登录校验共用 Redis OTP 约定）
type PhoneOTPDeps struct {
	Store    authentication.OTPCodeStore
	Gate     authentication.OTPSendGate
	SMS      authentication.SMSSender
	TTL      time.Duration // 验证码有效期，0 表示使用默认 5m
	Cooldown time.Duration // 同一号码发送间隔，0 表示使用默认 60s
	CodeLen  int           // 数字验证码长度，0 表示默认 6
}

func (d *PhoneOTPDeps) effectiveTTL() time.Duration {
	if d == nil || d.TTL <= 0 {
		return 5 * time.Minute
	}
	return d.TTL
}

func (d *PhoneOTPDeps) effectiveCooldown() time.Duration {
	if d == nil || d.Cooldown <= 0 {
		return 60 * time.Second
	}
	return d.Cooldown
}

func (d *PhoneOTPDeps) effectiveCodeLen() int {
	if d == nil || d.CodeLen <= 0 {
		return 6
	}
	if d.CodeLen > 12 {
		return 12
	}
	return d.CodeLen
}

// loginOTPScene 与 domain 层 PhoneOTPAuthStrategy 中 OTP 场景一致
const loginOTPScene = "login"

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
