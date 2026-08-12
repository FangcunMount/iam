package credential

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// ==================== Interface Interfaces (Driving Ports) ====================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// CredentialIssuer 凭据颁发器接口
type CredentialIssuer interface {
	// Issue 颁发凭据
	IssuePasswordCredential(req PasswordCredentialRequest) (*Credential, error)
}

// PasswordCredentialRequest 密码凭据颁发请求
type PasswordCredentialRequest struct {
	LoginIdentityID meta.ID // 登录身份ID
	PlainPassword   string  // 明文密码
	HashedPassword  string  // 已哈希的密码
	Algo            string  // 哈希算法
}

// ==================== PasswordHasher 接口 ====================

// PasswordHasher 密码哈希器接口（Driven Port）
// 由基础设施层实现，领域层使用
type PasswordHasher interface {
	Hash(plaintext string) (string, error) // 哈希明文密码
	Verify(hashed, plaintext string) bool  // 验证密码
	Pepper() string                        // 获取 Pepper
	NeedRehash(hashed string) bool         // 检查是否需要重新哈希
}
