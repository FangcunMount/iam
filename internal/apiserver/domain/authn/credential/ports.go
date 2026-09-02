package credential

// PasswordMaterialHasher 为密码凭据签发提供材料哈希能力。
type PasswordMaterialHasher interface {
	// 哈希密码
	Hash(plaintext string) (string, error)
	// 获取pepper
	Pepper() string
}
