package authentication

import "context"

// PasswordHasher 密码哈希服务（密码加密算法）。
// 职责：提供密码哈希验证和 rehash 能力。
type PasswordHasher interface {
	// 验证密码哈希是否匹配
	Verify(storedHash, plaintext string) bool
	// 是否需要重新哈希
	NeedRehash(storedHash string) bool
	// 哈希密码
	Hash(plaintext string) (string, error)
	// 获取pepper
	Pepper() string
}

// LoginPhoneOTPVerifier 登录短信验证码验证服务。
// 职责：验证登录 OTP 并消费（防止重放）。
type LoginPhoneOTPVerifier interface {
	// 验证并消费登录短信验证码
	VerifyAndConsumeLoginPhoneOTP(ctx context.Context, phoneE164, code string) bool
}
