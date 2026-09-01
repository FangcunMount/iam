package identity

import (
	"context"

	"gorm.io/gorm"

	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	sessionrevocation "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/sessionrevocation"
)

// EffectiveRoleReader resolves direct and inherited roles for a subject.
type EffectiveRoleReader interface {
	EffectiveRoleNamesForSubject(ctx context.Context, subject subject.Ref, tenantID string) ([]string, error)
}

// IdentityModuleDeps contains the runtime dependencies required to assemble the
// identity module.
type IdentityModuleDeps struct {
	DB                      *gorm.DB
	EffectiveRoles          EffectiveRoleReader
	SessionRevoker          sessiondomain.Revoker
	SessionRevocationConfig sessionrevocation.WorkerConfig
}
