package authz

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"gorm.io/gorm"
)

// AuthzModuleDeps contains the runtime dependencies required to assemble the
// authorization module.
type AuthzModuleDeps struct {
	SyncConfig                authzruntime.Config
	AttributeProvidersFile    string
	DB                        *gorm.DB
	EventStager               event.Stager
	GRPCACLEnabled            bool
	GRPCACLConfigFile         string
	AssignmentConstraintsFile string
	UserResolver              useraccess.UserResolver
}
