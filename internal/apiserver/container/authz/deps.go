package authz

import (
	"gorm.io/gorm"

	objectattributeadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

// AuthzModuleDeps contains the runtime dependencies required to assemble the
// authorization module.
type AuthzModuleDeps struct {
	DB                             *gorm.DB
	EventStager                    event.Stager
	GRPCACLEnabled                 bool
	GRPCACLConfigFile              string
	AssignmentConstraintsFile      string
	UserResolver                   useraccess.UserResolver
	ObjectAttributeAdmissionPolicy objectattributeadmission.Policy
}
