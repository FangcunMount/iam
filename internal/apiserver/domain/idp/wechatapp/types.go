package wechatapp

// AppType 微信应用类型
type AppType string

const (
	MiniProgram AppType = "MiniProgram" // 小程序
	MP          AppType = "MP"          // 公众号
	// OpenPlatformWebsite 微信开放平台网站应用（PC Web 扫码登录），
	// 与公众号网页授权(MP)、小程序登录(MiniProgram)是不同的身份源边界。
	OpenPlatformWebsite AppType = "OpenPlatformWebsite"
)

// String 获取类型字符串
func (t AppType) String() string {
	return string(t)
}

// IsValid 判断是否为受支持的微信应用类型。
func (t AppType) IsValid() bool {
	switch t {
	case MiniProgram, MP, OpenPlatformWebsite:
		return true
	default:
		return false
	}
}

// Status 微信应用状态
type Status string

const (
	StatusEnabled  Status = "Enabled"  // 已启用
	StatusDisabled Status = "Disabled" // 已禁用
	StatusArchived Status = "Archived" // 已归档
)

// CryptoAlg 加密算法
type CryptoAlg string

const (
	AlgAES256 CryptoAlg = "AES256" // 对称
	AlgSM4    CryptoAlg = "SM4"

	AlgRSA CryptoAlg = "RSA" // 非对称
	AlgSM2 CryptoAlg = "SM2"
)
