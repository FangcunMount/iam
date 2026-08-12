package credential

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// PasswordIssuer 为 LoginIdentity 创建 password Credential。
//
// Issuer 不负责持久化；应用层在事务中保存返回的 Credential。
type credentialIssuer struct {
	hasher PasswordHasher // 用于密码哈希
}

// 确保实现了接口
var _ CredentialIssuer = (*credentialIssuer)(nil)

// NewPasswordIssuer 创建 password credential issuer。
func NewPasswordIssuer(hasher PasswordHasher) *credentialIssuer {
	return &credentialIssuer{hasher: hasher}
}

// Issue 颁发密码凭据（创建凭据实体，不包含持久化）
func (i *credentialIssuer) IssuePasswordCredential(req PasswordCredentialRequest) (*Credential, error) {
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
func (i *credentialIssuer) hashPassword(plainPassword string) (string, error) {
	plaintextWithPepper := plainPassword + i.hasher.Pepper()
	return i.hasher.Hash(plaintextWithPepper)
}
