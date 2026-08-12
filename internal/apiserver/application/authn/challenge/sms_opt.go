package challenge

import "time"

// SMSOTP 短信验证码
type SMSOTP struct {
	Scene     string
	PhoneE164 string
	Code      string
	ExpiresAt time.Time
}

// SMSOTPOption 短信验证码选项
type SMSOTPOption func(*smsOTPOptions)

// smsOTPOptions 短信验证码选项
type smsOTPOptions struct {
	ttl     time.Duration
	codeLen int
	now     time.Time
}

// WithTTL 设置短信验证码有效期
func WithTTL(ttl time.Duration) SMSOTPOption {
	return func(o *smsOTPOptions) { o.ttl = ttl }
}

// WithCodeLen 设置短信验证码长度
func WithCodeLen(codeLen int) SMSOTPOption {
	return func(o *smsOTPOptions) { o.codeLen = codeLen }
}
