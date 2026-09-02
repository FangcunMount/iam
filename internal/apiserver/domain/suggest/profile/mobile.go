package profile

import (
	"strings"
	"unicode"
)

// LooksLikeMobile 对纯数字关键词做宽松手机号判断。
func LooksLikeMobile(digits string) bool {
	digits = strings.TrimSpace(digits)
	n := len(digits)
	if n < 7 || n > 15 {
		return false
	}
	for _, r := range digits {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// MaskMobile 脱敏手机号；长度不足返回空串。
func MaskMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	if len(mobile) < 7 {
		return ""
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
