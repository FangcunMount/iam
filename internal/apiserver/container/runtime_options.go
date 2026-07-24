package container

import (
	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
	genericoptions "github.com/FangcunMount/iam/v2/internal/pkg/options"
	genericapiserver "github.com/FangcunMount/iam/v2/internal/pkg/server"
)

// RuntimeOptions contains typed bootstrap options consumed by the container.
type RuntimeOptions struct {
	Environment                   genericapiserver.Environment
	Auth                          apiserveroptions.AuthOptions
	JWKS                          apiserveroptions.JWKSOptions
	IDP                           apiserveroptions.IDPOptions
	SMS                           apiserveroptions.SMSOptions
	Identity                      apiserveroptions.IdentityOptions
	Health                        apiserveroptions.HealthOptions
	Suggest                       appsuggest.Config
	Events                        apiserveroptions.EventOptions
	GRPCACLEnabled                bool
	GRPCACLConfigFile             string
	GRPCAssignmentConstraintsFile string
}

// RuntimeOptionsFromAPIServerOptions converts decoded apiserver options into
// the narrow config surface needed by module composition.
func RuntimeOptionsFromAPIServerOptions(opts *apiserveroptions.Options, environment genericapiserver.Environment) RuntimeOptions {
	defaults := apiserveroptions.NewOptions()
	if opts == nil {
		opts = defaults
	}

	runtime := RuntimeOptions{
		Environment:                   environment,
		Auth:                          *defaults.Auth,
		JWKS:                          *defaults.JWKS,
		IDP:                           *defaults.IDP,
		SMS:                           *defaults.SMS,
		Identity:                      *defaults.Identity,
		Health:                        *defaults.Health,
		Events:                        *defaults.Events,
		GRPCACLEnabled:                defaults.GRPCOptions.ACL != nil && defaults.GRPCOptions.ACL.Enabled,
		GRPCACLConfigFile:             grpcACLConfigFile(defaults.GRPCOptions),
		GRPCAssignmentConstraintsFile: defaults.GRPCOptions.AuthzAssignmentConstraintsFile,
		Suggest: appsuggest.Config{
			Enable:                    defaults.Suggest.Enable,
			Required:                  defaults.Suggest.Required,
			FullSyncCron:              defaults.Suggest.FullSyncCron,
			DeltaSyncCron:             defaults.Suggest.DeltaSyncCron,
			MaxResults:                defaults.Suggest.MaxResults,
			InternalMaxResults:        defaults.Suggest.InternalMaxResults,
			KeyPadLen:                 defaults.Suggest.KeyPadLen,
			FullSQL:                   defaults.Suggest.FullSQL,
			DeltaSQL:                  defaults.Suggest.DeltaSQL,
			DisableMobileMask:         defaults.Suggest.DisableMobileMask,
			LoaderPlaceholderOrgID:    defaults.Suggest.LoaderPlaceholderOrgID,
			LoaderPlaceholderTenantID: defaults.Suggest.LoaderPlaceholderTenantID,
			WildcardKeyCap:            defaults.Suggest.WildcardKeyCap,
			VisibilityCacheTTLSeconds: defaults.Suggest.VisibilityCacheTTLSeconds,
			RateLimit:                 suggestRateLimitConfig(*defaults.Suggest),
		}.WithDefaults(),
	}
	if opts.Auth != nil {
		runtime.Auth = *opts.Auth
	}
	if opts.JWKS != nil {
		runtime.JWKS = *opts.JWKS
	}
	if opts.IDP != nil {
		runtime.IDP = *opts.IDP
	}
	if opts.SMS != nil {
		runtime.SMS = *opts.SMS
	}
	if opts.Identity != nil {
		runtime.Identity = *opts.Identity
	}
	if opts.Health != nil {
		runtime.Health = *opts.Health
	}
	if opts.Events != nil {
		runtime.Events = *opts.Events
	}
	if opts.GRPCOptions != nil {
		runtime.GRPCAssignmentConstraintsFile = opts.GRPCOptions.AuthzAssignmentConstraintsFile
		if opts.GRPCOptions.ACL != nil {
			runtime.GRPCACLEnabled = opts.GRPCOptions.ACL.Enabled
			runtime.GRPCACLConfigFile = opts.GRPCOptions.ACL.ConfigFile
		}
	}
	if opts.Suggest != nil {
		runtime.Suggest = appsuggest.Config{
			Enable:                    opts.Suggest.Enable,
			Required:                  opts.Suggest.Required,
			FullSyncCron:              opts.Suggest.FullSyncCron,
			DeltaSyncCron:             opts.Suggest.DeltaSyncCron,
			MaxResults:                opts.Suggest.MaxResults,
			InternalMaxResults:        opts.Suggest.InternalMaxResults,
			KeyPadLen:                 opts.Suggest.KeyPadLen,
			FullSQL:                   opts.Suggest.FullSQL,
			DeltaSQL:                  opts.Suggest.DeltaSQL,
			DisableMobileMask:         opts.Suggest.DisableMobileMask,
			LoaderPlaceholderOrgID:    opts.Suggest.LoaderPlaceholderOrgID,
			LoaderPlaceholderTenantID: opts.Suggest.LoaderPlaceholderTenantID,
			WildcardKeyCap:            opts.Suggest.WildcardKeyCap,
			VisibilityCacheTTLSeconds: opts.Suggest.VisibilityCacheTTLSeconds,
			RateLimit:                 suggestRateLimitConfig(*opts.Suggest),
		}.WithDefaults()
	}
	if runtime.Events.CatalogPath == "" {
		runtime.Events.CatalogPath = defaults.Events.CatalogPath
	}
	if runtime.Events.OutboxRelayInterval <= 0 {
		runtime.Events.OutboxRelayInterval = defaults.Events.OutboxRelayInterval
	}
	return runtime
}

func grpcACLConfigFile(options *genericoptions.GRPCOptions) string {
	if options == nil || options.ACL == nil {
		return ""
	}
	return options.ACL.ConfigFile
}

func suggestRateLimitConfig(o apiserveroptions.SuggestOptions) appsuggest.RateLimitConfig {
	return appsuggest.RateLimitConfig{
		PerOperatorQPS:                o.RateLimit.PerOperatorQPS,
		PerOperatorBurst:              o.RateLimit.PerOperatorBurst,
		MobileKeywordPerOperatorQPS:   o.RateLimit.MobileKeywordPerOperatorQPS,
		MobileKeywordPerOperatorBurst: o.RateLimit.MobileKeywordPerOperatorBurst,
		Backend:                       o.RateLimit.Backend,
		OperatorMapMaxEntries:         o.RateLimit.OperatorMapMaxEntries,
	}
}
