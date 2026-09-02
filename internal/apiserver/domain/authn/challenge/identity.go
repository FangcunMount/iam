package challenge

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// SMSOTPChallengeID 构造短信验证码挑战 ID。
func SMSOTPChallengeID(scene, phoneE164 string) string {
	return fmt.Sprintf("sms_otp:%s:%s", strings.TrimSpace(scene), strings.TrimSpace(phoneE164))
}

// SMSOTPSecretHash 计算短信验证码挑战密钥哈希。
func SMSOTPSecretHash(scene, phoneE164, otp string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(scene) + "\x00" + strings.TrimSpace(phoneE164) + "\x00" + strings.TrimSpace(otp)))
	return sum[:]
}

// OAuthStateChallengeID 构造 OAuth state 挑战 ID。
func OAuthStateChallengeID(scene, state string) string {
	return fmt.Sprintf("oauth_state:%s:%s", strings.TrimSpace(scene), strings.TrimSpace(state))
}

// OAuthStateSecretHash 计算 OAuth state 挑战密钥哈希。
func OAuthStateSecretHash(state string) []byte {
	sum := sha256.Sum256([]byte("oauth_state\x00" + strings.TrimSpace(state)))
	return sum[:]
}
