package loginidentity

import (
	"encoding/json"
	"time"

	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
)

type Mapper struct{}

func NewMapper() *Mapper { return &Mapper{} }

func (m *Mapper) ToPO(identity *domain.LoginIdentity) *PO {
	if identity == nil {
		return nil
	}
	po := &PO{
		UserID:     identity.UserID,
		Provider:   string(identity.Provider),
		Realm:      identity.Realm,
		Identifier: identity.Identifier,
		Status:     string(identity.Status),
		VerifiedAt: copyTimePtr(identity.VerifiedAt),
		LinkedAt:   identity.LinkedAt,
		Profile:    mapToJSON(identity.Profile),
		Meta:       mapToJSON(identity.Meta),
	}
	if !identity.ID.IsZero() {
		po.ID = identity.ID
	}
	if identity.GlobalIdentifier != "" {
		globalIdentifier := identity.GlobalIdentifier
		po.GlobalIdentifier = &globalIdentifier
	}
	return po
}

func (m *Mapper) ToDO(po *PO) *domain.LoginIdentity {
	if po == nil {
		return nil
	}
	identity := &domain.LoginIdentity{
		ID:         po.ID,
		UserID:     po.UserID,
		Provider:   domain.Provider(po.Provider),
		Realm:      po.Realm,
		Identifier: po.Identifier,
		Status:     domain.Status(po.Status),
		VerifiedAt: copyTimePtr(po.VerifiedAt),
		LinkedAt:   po.LinkedAt,
		Profile:    jsonToMap(po.Profile),
		Meta:       jsonToMap(po.Meta),
		CreatedAt:  po.CreatedAt,
		UpdatedAt:  po.UpdatedAt,
	}
	if po.GlobalIdentifier != nil {
		identity.GlobalIdentifier = *po.GlobalIdentifier
	}
	return identity
}

func mapToJSON(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}
	data, _ := json.Marshal(m)
	return data
}

func jsonToMap(data []byte) map[string]string {
	if len(data) == 0 {
		return map[string]string{}
	}
	var m map[string]string
	_ = json.Unmarshal(data, &m)
	if m == nil {
		return map[string]string{}
	}
	return m
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	t := *src
	return &t
}
