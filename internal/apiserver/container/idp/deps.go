package idp

import (
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// IDPModuleDeps contains the runtime dependencies required to assemble the IDP module.
type IDPModuleDeps struct {
	DB            *gorm.DB
	RedisClient   *redis.Client
	EncryptionKey []byte
}

func validateIDPModuleDeps(deps IDPModuleDeps) error {
	if deps.DB == nil {
		log.Warnf("IDP module initialization requires a valid database connection")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"database connection is nil or invalid")
	}

	if deps.RedisClient == nil {
		log.Warnf("IDP module initialization requires a valid Redis client")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"redis client is nil or invalid")
	}

	if len(deps.EncryptionKey) != 32 {
		log.Warnf("IDP module initialization requires a 32-byte encryption key")
		return errors.WithCode(code.ErrModuleInitializationFailed,
			"encryption key must be 32 bytes for AES-256")
	}

	return nil
}
