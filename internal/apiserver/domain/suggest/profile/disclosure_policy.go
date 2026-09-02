package profile

// MobileDisclosurePolicy 控制手机号明文/脱敏输出。
type MobileDisclosurePolicy struct {
	DisableMask bool
}

// Disclose 返回展示用手机号字符串。
func (p MobileDisclosurePolicy) Disclose(mobiles []string) string {
	if len(mobiles) == 0 {
		return ""
	}
	if p.DisableMask {
		return mobiles[0]
	}
	return MaskMobile(mobiles[0])
}
