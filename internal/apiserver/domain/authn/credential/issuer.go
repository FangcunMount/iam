package credential

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// IssuePasswordRequest 密码凭据颁发请求
type IssuePasswordRequest struct {
	LoginIdentityID meta.ID // 登录身份ID
	PlainPassword   string  // 明文密码（必须）
	HashedPassword  string  // 已哈希的密码（可选，如果提供则直接使用，不再哈希）
	Algo            string  // 哈希算法（如果使用 HashedPassword，必须提供）
}

// PasswordIssuer 为 LoginIdentity 创建 password Credential。
//
// Issuer 不负责持久化；应用层在事务中保存返回的 Credential。
type PasswordIssuer struct {
	hasher PasswordHasher // 用于密码哈希
}

// NewPasswordIssuer 创建 password credential issuer。
func NewPasswordIssuer(hasher PasswordHasher) *PasswordIssuer {
	return &PasswordIssuer{hasher: hasher}
}

// IssuePassword 颁发密码凭据（创建凭据实体，不包含持久化）
func (i *PasswordIssuer) IssuePassword(req IssuePasswordRequest) (*Credential, error) {
	// 参数验证
	if req.LoginIdentityID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login_identity_id is required")
	}
	if req.PlainPassword == "" && req.HashedPassword == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "plain_password or hashed_password is required")
	}

	var hashedPassword string
	var algo string

	// 如果提供了已哈希的密码，直接使用
	if req.HashedPassword != "" {
		if req.Algo == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "algo is required when using hashed_password")
		}
		hashedPassword = req.HashedPassword
		algo = req.Algo
	} else {
		// 哈希明文密码（PHC 格式）
		var err error
		hashedPassword, err = i.hashPassword(req.PlainPassword)
		if err != nil {
			return nil, perrors.WithCode(code.ErrEncrypt, "failed to hash password: %v", err)
		}
		algo = "argon2id"
	}

	if hashedPassword == "" {
		return nil, perrors.WithCode(code.ErrInvalidCredential, "password credential requires material")
	}
	if algo == "" {
		return nil, perrors.WithCode(code.ErrInvalidCredential, "password credential requires algo")
	}

	return NewPasswordCredential(req.LoginIdentityID, []byte(hashedPassword), algo), nil
}

// hashPassword 使用 PHC 格式哈希密码
func (i *PasswordIssuer) hashPassword(plainPassword string) (string, error) {
	plaintextWithPepper := plainPassword + i.hasher.Pepper()
	return i.hasher.Hash(plaintextWithPepper)
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
