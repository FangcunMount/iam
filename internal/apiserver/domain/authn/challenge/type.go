package challenge

// ChallengeType 挑战类型
type ChallengeType string

const (
	TypeSMSOTP     ChallengeType = "sms_otp"     // 短信验证码
	TypeEmailOTP   ChallengeType = "email_otp"   // 邮箱验证码
	TypeOAuthState ChallengeType = "oauth_state" // OAuth状态
	TypeLoginCode  ChallengeType = "login_code"  // 登录验证码
)
