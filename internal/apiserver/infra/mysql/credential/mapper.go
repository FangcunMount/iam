package credential

import (
	"time"

	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	base "github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
)

type Mapper struct{}

func NewMapper() *Mapper { return &Mapper{} }

func (m *Mapper) ToPO(cred *domain.Credential) *V2PO {
	if cred == nil {
		return nil
	}
	return &V2PO{
		AuditFields:     auditFieldsFromCredential(cred),
		LoginIdentityID: cred.LoginIdentityID,
		Type:            string(cred.Type),
		Material:        cloneBytes(cred.Material),
		Algo:            copyStringPtr(cred.Algo),
		Params:          cloneBytes(cred.ParamsJSON),
		Status:          cred.Status.String(),
		FailedAttempts:  cred.FailedAttempts,
		LockedUntil:     copyTimePtr(cred.LockedUntil),
		LastSuccessAt:   copyTimePtr(cred.LastSuccessAt),
		LastFailureAt:   copyTimePtr(cred.LastFailureAt),
	}
}

func (m *Mapper) ToDO(po *V2PO) *domain.Credential {
	if po == nil {
		return nil
	}
	return &domain.Credential{
		ID:              po.ID,
		LoginIdentityID: po.LoginIdentityID,
		Type:            domain.CredentialType(po.Type),
		Material:        cloneBytes(po.Material),
		Algo:            po.Algo,
		ParamsJSON:      cloneBytes(po.Params),
		Status:          statusFromString(po.Status),
		FailedAttempts:  po.FailedAttempts,
		LockedUntil:     copyTimePtr(po.LockedUntil),
		LastSuccessAt:   copyTimePtr(po.LastSuccessAt),
		LastFailureAt:   copyTimePtr(po.LastFailureAt),
	}
}

func auditFieldsFromCredential(cred *domain.Credential) base.AuditFields {
	if cred == nil || cred.ID.IsZero() {
		return base.AuditFields{}
	}
	return base.AuditFields{ID: cred.ID}
}

func statusFromString(status string) domain.CredentialStatus {
	switch status {
	case domain.CredStatusEnabled.String():
		return domain.CredStatusEnabled
	default:
		return domain.CredStatusDisabled
	}
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
