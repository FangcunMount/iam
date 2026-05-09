package challenge

type ChallengeType string

const (
	TypeSMSOTP     ChallengeType = "sms_otp"
	TypeEmailOTP   ChallengeType = "email_otp"
	TypeOAuthState ChallengeType = "oauth_state"
	TypeLoginCode  ChallengeType = "login_code"
)
