package credential

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// binder 凭据绑定器实现
// 职责：将外部认证信息绑定到账号，创建凭据实体
type binder struct{}

// 确保实现了 Binder 接口
var _ Binder = (*binder)(nil)

// NewBinder 创建凭据绑定器实例
func NewBinder() Binder {
	return &binder{}
}

// Bind 绑定凭据到账号
// 根据规范创建凭据实体，不涉及持久化
func (b *binder) Bind(spec BindSpec) (*Credential, error) {
	// 参数校验
	if spec.AccountID.IsZero() {
		return nil, errors.WithCode(code.ErrInvalidArgument, "account ID cannot be zero")
	}
	if spec.Type == "" {
		return nil, errors.WithCode(code.ErrInvalidArgument, "credential type cannot be empty")
	}

	// 创建凭据实体
	cred := &Credential{
		AccountID:     spec.AccountID,
		IDP:           spec.IDP,
		IDPIdentifier: spec.IDPIdentifier,
		AppID:         spec.AppID,
		Material:      spec.Material,
		Algo:          spec.Algo,
		ParamsJSON:    spec.ParamsJSON,
		Status:        CredStatusEnabled, // 默认启用
	}

	// 根据类型验证必需字段
	switch spec.Type {
	case CredPassword:
		// password 类型需要 Material 和 Algo
		if len(spec.Material) == 0 {
			return nil, errors.WithCode(code.ErrInvalidCredential, "password credential requires material")
		}
		if spec.Algo == nil || *spec.Algo == "" {
			return nil, errors.WithCode(code.ErrInvalidCredential, "password credential requires algo")
		}
		// password 类型不需要 IDP
		cred.IDP = nil
		cred.AppID = nil

	case CredPhoneOTP:
		// phone_otp 需要 IDPIdentifier（手机号）
		if spec.IDPIdentifier == "" {
			return nil, errors.WithCode(code.ErrInvalidCredential, "phone_otp credential requires IDP identifier (phone number)")
		}
		// phone_otp 不需要 Material
		cred.Material = nil
		cred.Algo = nil

	case CredOAuthWxMinip, CredOAuthWecom:
		// OAuth 类型需要 IDPIdentifier 和 AppID
		if spec.IDPIdentifier == "" {
			return nil, errors.WithCode(code.ErrInvalidCredential, "OAuth credential requires IDP identifier")
		}
		if spec.AppID == nil || *spec.AppID == "" {
			return nil, errors.WithCode(code.ErrInvalidCredential, "OAuth credential requires AppID")
		}
		// OAuth 不需要 Material
		cred.Material = nil
		cred.Algo = nil

	default:
		return nil, errors.WithCode(code.ErrInvalidCredential, "unsupported credential type: %s", spec.Type)
	}

	return cred, nil
}
