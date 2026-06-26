package identity

import (
	"context"

	"gorm.io/gorm"

	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
)

// RoleNameReader resolves role names for a subject within a tenant.
type RoleNameReader interface {
	RoleNamesForSubject(ctx context.Context, subject subject.Ref, tenantID string) ([]string, error)
}

// IdentityModuleDeps contains the runtime dependencies required to assemble the
// identity module.
type IdentityModuleDeps struct {
	DB             *gorm.DB
	RoleNames      RoleNameReader
	SessionRevoker sessiondomain.Revoker
}
