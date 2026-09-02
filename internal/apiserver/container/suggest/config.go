package suggest

import (
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	suggestmemory "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/index/memory"
	mysqlsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/suggest"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
	suggestvisibility "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/visibility"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

// ModuleConfig 是 composition root 的 suggest 模块配置。
type ModuleConfig struct {
	Enable        bool
	Required      bool
	FullSyncCron  string
	DeltaSyncCron string
	Query         appquery.Config
	Memory        suggestmemory.Config
	Loader        mysqlsuggest.LoaderConfig
	Visibility    suggestvisibility.Config
	RateLimit     suggestratelimit.Config
}

// ModuleConfigFromOptions 从外部 SuggestOptions 映射各层配置。
func ModuleConfigFromOptions(o apiserveroptions.SuggestOptions) ModuleConfig {
	query := appquery.Config{
		MaxResults:        o.MaxResults,
		CandidateBudget:   o.InternalMaxResults,
		DisableMobileMask: o.DisableMobileMask,
	}.WithDefaults()

	fullCron := o.FullSyncCron
	if fullCron == "" {
		fullCron = "@every 1h"
	}

	return ModuleConfig{
		Enable:        o.Enable,
		Required:      o.Required,
		FullSyncCron:  fullCron,
		DeltaSyncCron: o.DeltaSyncCron,
		Query:         query,
		Memory: suggestmemory.Config{
			KeyPadLen:      o.KeyPadLen,
			WildcardKeyCap: o.WildcardKeyCap,
		}.WithDefaults(),
		Loader: mysqlsuggest.LoaderConfig{
			FullSQL:          o.FullSQL,
			DeltaSQL:         o.DeltaSQL,
			PlaceholderOrgID: o.LoaderPlaceholderOrgID,
		},
		Visibility: suggestvisibility.Config{CacheTTLSeconds: o.VisibilityCacheTTLSeconds},
		RateLimit:  suggestratelimit.ConfigFromOptions(o),
	}
}

// SuggestModuleDeps contains the runtime dependencies required to assemble the suggest module.
type SuggestModuleDeps struct {
	DB                     *gorm.DB
	Config                 ModuleConfig
	RoutePermissionChecker authorizationapp.RoutePermissionChecker
	Environment            genericapiserver.Environment
	RedisClient            *redis.Client
}
