package session

import (
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// New keeps legacy test fixtures concise without reintroducing the old production constructor.
func New(sessionID string, userID, loginIdentityID, tenantID meta.ID, amr []string, _ map[string]string, expiresAt time.Time) *Session {
	methods := make([]authentication.AMR, 0, len(amr))
	for _, value := range amr {
		methods = append(methods, authentication.AMR(value))
	}
	return NewWithContexts(
		sessionID, userID, loginIdentityID, tenantID,
		authentication.RestoreAuthenticationContext("", "", methods, time.Time{}),
		authentication.TokenContext{}, expiresAt,
	)
}
