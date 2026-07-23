package authn

import (
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

// AuthnModuleDeps contains the runtime dependencies required to assemble the
// authentication module.
type AuthnModuleDeps struct {
	DB             *gorm.DB
	RedisClient    *redis.Client
	PasswordHasher authentication.PasswordHasher
	IDPModule      *idp.IDPModule
	EventBus       messaging.EventBus
	EventPublisher event.Publisher
	Environment    genericapiserver.Environment
	Auth           apiserveroptions.AuthOptions
	JWKS           apiserveroptions.JWKSOptions
	IDPOptions     apiserveroptions.IDPOptions
	SMS            apiserveroptions.SMSOptions
}
