package credential

import (
	"time"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
)

// Mapper 负责领域模型与持久化对象之间的转换。
type Mapper struct{}

// NewMapper 创建新的映射器实例。
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToPO 将凭据领域模型转换为持久化对象。
func (m *Mapper) ToPO(cred *domain.Credential) *PO {
	if cred == nil {
		return nil
	}

	credType := inferCredentialType(cred)

	po := &PO{
		AccountID:      cred.AccountID,
		Type:           string(credType),
		IDP:            copyStringPtr(cred.IDP),
		IDPIdentifier:  cred.IDPIdentifier,
		AppID:          copyStringPtr(cred.AppID),
		Material:       cloneBytes(cred.Material),
		Algo:           copyStringPtr(cred.Algo),
		Params:         cloneBytes(cred.ParamsJSON),
		Status:         int8(cred.Status),
		FailedAttempts: cred.FailedAttempts,
		LockedUntil:    copyTimePtr(cred.LockedUntil),
		LastSuccessAt:  copyTimePtr(cred.LastSuccessAt),
		LastFailureAt:  copyTimePtr(cred.LastFailureAt),
	}

	if !cred.ID.IsZero() {
		po.ID = cred.ID
	}

	return po
}

// ToDO 将持久化对象转换为凭据领域模型。
func (m *Mapper) ToDO(po *PO) *domain.Credential {
	if po == nil {
		return nil
	}

	return &domain.Credential{
		ID:             po.ID,
		AccountID:      po.AccountID,
		IDP:            po.IDP,
		IDPIdentifier:  po.IDPIdentifier,
		AppID:          po.AppID,
		Material:       cloneBytes(po.Material),
		Algo:           po.Algo,
		ParamsJSON:     cloneBytes(po.Params),
		Status:         domain.CredentialStatus(po.Status),
		FailedAttempts: po.FailedAttempts,
		LockedUntil:    copyTimePtr(po.LockedUntil),
		LastSuccessAt:  copyTimePtr(po.LastSuccessAt),
		LastFailureAt:  copyTimePtr(po.LastFailureAt),
	}
}

func inferCredentialType(cred *domain.Credential) domain.CredentialType {
	if cred.IDP == nil && len(cred.Material) > 0 && cred.Algo != nil {
		return domain.CredPassword
	}
	if cred.IDP != nil && *cred.IDP == "phone" {
		return domain.CredPhoneOTP
	}
	if cred.IDP != nil && *cred.IDP == "wechat" {
		return domain.CredOAuthWxMinip
	}
	if cred.IDP != nil && *cred.IDP == "wecom" {
		return domain.CredOAuthWecom
	}
	return domain.CredPassword
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func copyStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	s := *src
	return &s
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	t := *src
	return &t
}
