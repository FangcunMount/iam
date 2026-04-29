package container

import (
	appsuggest "github.com/FangcunMount/iam/internal/apiserver/application/suggest"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
)

// RuntimeOptions contains typed bootstrap options consumed by the container.
type RuntimeOptions struct {
	AppMode string
	Auth    apiserveroptions.AuthOptions
	JWKS    apiserveroptions.JWKSOptions
	SMS     apiserveroptions.SMSOptions
	Suggest appsuggest.Config
	Events  apiserveroptions.EventOptions
}

// RuntimeOptionsFromAPIServerOptions converts decoded apiserver options into
// the narrow config surface needed by module composition.
func RuntimeOptionsFromAPIServerOptions(opts *apiserveroptions.Options, appMode string) RuntimeOptions {
	defaults := apiserveroptions.NewOptions()
	if opts == nil {
		opts = defaults
	}

	runtime := RuntimeOptions{
		AppMode: appMode,
		Auth:    *defaults.Auth,
		JWKS:    *defaults.JWKS,
		SMS:     *defaults.SMS,
		Events:  *defaults.Events,
		Suggest: appsuggest.Config{
			Enable:        defaults.Suggest.Enable,
			DataDir:       defaults.Suggest.DataDir,
			FullSyncCron:  defaults.Suggest.FullSyncCron,
			DeltaSyncCron: defaults.Suggest.DeltaSyncCron,
			MaxResults:    defaults.Suggest.MaxResults,
			KeyPadLen:     defaults.Suggest.KeyPadLen,
			FullSQL:       defaults.Suggest.FullSQL,
			DeltaSQL:      defaults.Suggest.DeltaSQL,
			Snapshot:      suggestSnapshot(*defaults.Suggest),
		}.WithDefaults(),
	}
	if opts.Auth != nil {
		runtime.Auth = *opts.Auth
	}
	if opts.JWKS != nil {
		runtime.JWKS = *opts.JWKS
	}
	if opts.SMS != nil {
		runtime.SMS = *opts.SMS
	}
	if opts.Events != nil {
		runtime.Events = *opts.Events
	}
	if opts.Suggest != nil {
		runtime.Suggest = appsuggest.Config{
			Enable:        opts.Suggest.Enable,
			DataDir:       opts.Suggest.DataDir,
			FullSyncCron:  opts.Suggest.FullSyncCron,
			DeltaSyncCron: opts.Suggest.DeltaSyncCron,
			MaxResults:    opts.Suggest.MaxResults,
			KeyPadLen:     opts.Suggest.KeyPadLen,
			FullSQL:       opts.Suggest.FullSQL,
			DeltaSQL:      opts.Suggest.DeltaSQL,
			Snapshot:      suggestSnapshot(*opts.Suggest),
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

func suggestSnapshot(opts apiserveroptions.SuggestOptions) bool {
	if opts.Snapshot != nil {
		return *opts.Snapshot
	}
	return opts.DataDir != ""
}
