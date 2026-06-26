package suggest

import (
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	authn "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
)

// SuggestModuleDeps contains the runtime dependencies required to assemble the
// suggest module.
type SuggestModuleDeps struct {
	DB                 *gorm.DB
	Config             appsuggest.Config
	RouteAuthorization authn.RouteAuthorizationRuntime
	AppMode            string
	RedisClient        *redis.Client
}
