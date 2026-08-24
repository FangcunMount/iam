package identity

import (
	"gorm.io/gorm"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	mysqluser "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/user"
)

// UserAccessCapabilities are Identity-owned facts published to sibling modules.
type UserAccessCapabilities struct {
	UserStatusReader useraccess.UserStatusReader
	UserResolver     useraccess.UserResolver
}

// NewUserAccessCapabilities assembles the narrow Identity boundary once at the composition root.
func NewUserAccessCapabilities(db *gorm.DB) UserAccessCapabilities {
	service := useraccess.NewService(mysqluser.NewRepository(db))
	return UserAccessCapabilities{
		UserStatusReader: service,
		UserResolver:     service,
	}
}
