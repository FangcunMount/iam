package authz

import (
	"gorm.io/gorm"

	"github.com/FangcunMount/iam/v2/pkg/event"
)

// AuthzModuleDeps contains the runtime dependencies required to assemble the
// authorization module.
type AuthzModuleDeps struct {
	DB          *gorm.DB
	EventStager event.Stager
	ModelPath   string
}
