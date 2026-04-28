package credential

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ==================== 凭据颁发器接口 ====================

// Issuer 凭据颁发器接口（领域服务）
// 职责：为账户颁发各种类型的登录凭据
// 注意：与 Binder 不同，Issuer 是面向应用层的领域服务，包含持久化逻辑
type Issuer interface {
	// IssuePassword 颁发密码凭据
	IssuePassword(ctx context.Context, req IssuePasswordRequest) (*Credential, error)

	// IssuePhoneOTP 颁发手机OTP凭据
	IssuePhoneOTP(ctx context.Context, req IssuePhoneOTPRequest) (*Credential, error)

	// IssueWechatMinip 颁发微信小程序凭据
	IssueWechatMinip(ctx context.Context, req IssueOAuthRequest) (*Credential, error)

	// IssueWecom 颁发企业微信凭据
	IssueWecom(ctx context.Context, req IssueOAuthRequest) (*Credential, error)
}

// ==================== 颁发请求 DTOs ====================

// IssuePasswordRequest 密码凭据颁发请求
type IssuePasswordRequest struct {
	AccountID      meta.ID // 账户ID（必须）
	PlainPassword  string  // 明文密码（必须）
	HashedPassword string  // 已哈希的密码（可选，如果提供则直接使用，不再哈希）
	Algo           string  // 哈希算法（如果使用 HashedPassword，必须提供）
}

// IssuePhoneOTPRequest 手机OTP凭据颁发请求
type IssuePhoneOTPRequest struct {
	AccountID meta.ID    // 账户ID（必须）
	Phone     meta.Phone // 手机号（必须）
}

// IssueOAuthRequest OAuth类凭据颁发请求（微信小程序、企业微信等）
type IssueOAuthRequest struct {
	AccountID     meta.ID // 账户ID（必须）
	IDP           string  // 第三方身份提供商（可选，有默认值）
	IDPIdentifier string  // 第三方标识符（OpenID/UnionID/UserID等，必须）
	AppID         string  // 应用ID（必须）
	ParamsJSON    []byte  // 第三方返回的JSON（可选）
}

// ==================== 颁发器实现 ====================

// issuer 凭据颁发器实现
type issuer struct {
	binder Binder
	hasher PasswordHasher // 用于密码哈希
}

var _ Issuer = (*issuer)(nil)

// NewIssuer 创建凭据颁发器
// 注意：不再依赖 Repository，持久化由应用层负责
func NewIssuer(hasher PasswordHasher) Issuer {
	return &issuer{
		binder: NewBinder(),
		hasher: hasher,
	}
}

// IssuePassword 颁发密码凭据（创建凭据实体，不包含持久化）
func (i *issuer) IssuePassword(ctx context.Context, req IssuePasswordRequest) (*Credential, error) {
	// 参数验证
	if req.AccountID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "account_id is required")
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

	// 创建凭据实体
	credential, err := i.binder.Bind(BindSpec{
		AccountID: req.AccountID,
		Type:      CredPassword,
		Material:  []byte(hashedPassword),
		Algo:      &algo,
	})
	if err != nil {
		return nil, err
	}

	// 注意：凭据持久化由应用层负责
	return credential, nil
}

// IssuePhoneOTP 颁发手机OTP凭据（创建凭据实体，不包含持久化）
func (i *issuer) IssuePhoneOTP(ctx context.Context, req IssuePhoneOTPRequest) (*Credential, error) {
	// 参数验证
	if req.AccountID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "account_id is required")
	}
	if req.Phone.IsEmpty() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "phone is required")
	}

	// 创建手机OTP凭据
	// 创建手机OTP凭据实体
	idp := "phone"
	credential, err := i.binder.Bind(BindSpec{
		AccountID:     req.AccountID,
		Type:          CredPhoneOTP,
		IDP:           &idp,
		IDPIdentifier: req.Phone.String(),
	})
	if err != nil {
		return nil, err
	}

	// 注意：凭据持久化由应用层负责
	return credential, nil
}

// IssueWechatMinip 颁发微信小程序凭据（创建凭据实体，不包含持久化）
func (i *issuer) IssueWechatMinip(ctx context.Context, req IssueOAuthRequest) (*Credential, error) {
	// 参数验证
	if req.AccountID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "account_id is required")
	}
	if req.IDPIdentifier == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "idp_identifier is required")
	}
	if req.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app_id is required")
	}

	// 设置默认 IDP
	if req.IDP == "" {
		req.IDP = "wechat"
	}

	// 创建微信凭据实体
	credential, err := i.binder.Bind(BindSpec{
		AccountID:     req.AccountID,
		Type:          CredOAuthWxMinip,
		IDP:           &req.IDP,
		IDPIdentifier: req.IDPIdentifier,
		AppID:         &req.AppID,
		ParamsJSON:    req.ParamsJSON,
	})
	if err != nil {
		return nil, err
	}

	// 注意：凭据持久化由应用层负责
	return credential, nil
}

// IssueWecom 颁发企业微信凭据（创建凭据实体，不包含持久化）
func (i *issuer) IssueWecom(ctx context.Context, req IssueOAuthRequest) (*Credential, error) {
	// 参数验证
	if req.AccountID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "account_id is required")
	}
	if req.IDPIdentifier == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "idp_identifier is required")
	}
	if req.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app_id is required")
	}

	// 设置默认 IDP
	if req.IDP == "" {
		req.IDP = "wecom"
	}

	// 创建企业微信凭据实体
	credential, err := i.binder.Bind(BindSpec{
		AccountID:     req.AccountID,
		Type:          CredOAuthWecom,
		IDP:           &req.IDP,
		IDPIdentifier: req.IDPIdentifier,
		AppID:         &req.AppID,
		ParamsJSON:    req.ParamsJSON,
	})
	if err != nil {
		return nil, err
	}

	// 注意：凭据持久化由应用层负责
	return credential, nil
}

// hashPassword 使用 PHC 格式哈希密码
func (i *issuer) hashPassword(plainPassword string) (string, error) {
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
