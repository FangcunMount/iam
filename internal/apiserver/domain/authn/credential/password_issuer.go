package credential

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// PasswordCredentialRequest 密码凭据颁发请求。
type PasswordCredentialRequest struct {
	// 登录身份ID
	LoginIdentityID meta.ID
	// 明文密码
	PlainPassword string
	// 哈希密码
	HashedPassword string
	// 算法
	Algo string
}

// PasswordIssuer 为 LoginIdentity 创建 password Credential。
//
// Issuer 不负责持久化；应用层在事务中保存返回的 Credential。
type PasswordIssuer struct {
	hasher PasswordMaterialHasher
}

// NewPasswordIssuer 创建 password credential issuer。
func NewPasswordIssuer(hasher PasswordMaterialHasher) *PasswordIssuer {
	return &PasswordIssuer{hasher: hasher}
}

// IssuePasswordCredential 颁发密码凭据（创建凭据实体，不包含持久化）。
func (i *PasswordIssuer) IssuePasswordCredential(req PasswordCredentialRequest) (*Credential, error) {
	// 检查登录身份 与 密码 是否为空
	if req.LoginIdentityID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login_identity_id is required")
	}
	if req.PlainPassword == "" && req.HashedPassword == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "plain_password or hashed_password is required")
	}

	// 确保密码已经哈希过
	hashedPassword, algo, err := i.ensureHashedPassword(req.PlainPassword, req.HashedPassword, req.Algo)
	if err != nil {
		return nil, err
	}

	// 创建密码凭据
	return NewPasswordCredential(req.LoginIdentityID, []byte(hashedPassword), algo), nil
}

// ensureHashedPassword 确保密码已经哈希过
func (i *PasswordIssuer) ensureHashedPassword(plainPassword, hashedPassword, algo string) (string, string, error) {
	// 如果哈希密码不为空，则直接使用
	if hashedPassword != "" {
		if algo == "" {
			return "", "", perrors.WithCode(code.ErrInvalidArgument, "algo is required when using hashed_password")
		}
		return hashedPassword, algo, nil
	}

	// 明文密码始终使用当前密码哈希器对应的 argon2id 算法。
	algo = "argon2id"

	// 明文密码加盐哈希
	plaintextWithPepper := plainPassword + i.hasher.Pepper()

	// 哈希密码
	hashed, err := i.hasher.Hash(plaintextWithPepper)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrEncrypt, "failed to hash password: %v", err)
	}
	if hashed == "" {
		return "", "", perrors.WithCode(code.ErrInvalidCredential, "password credential requires material")
	}
	return hashed, algo, nil
}
