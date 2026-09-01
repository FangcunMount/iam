package suggest

import (
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

// SuggestModuleDeps contains the runtime dependencies required to assemble the
// suggest module.
type SuggestModuleDeps struct {
	DB                     *gorm.DB
	Config                 appsuggest.Config
	RoutePermissionChecker authorizationapp.RoutePermissionChecker
	Environment            genericapiserver.Environment
	RedisClient            *redis.Client
}
