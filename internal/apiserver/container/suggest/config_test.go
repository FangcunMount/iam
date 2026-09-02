package suggest

import (
	"testing"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	suggestmemory "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/index/memory"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
)

func TestModuleConfigFromOptionsDefaults(t *testing.T) {
	cfg := ModuleConfigFromOptions(apiserveroptions.SuggestOptions{Enable: true})

	if cfg.FullSyncCron != "@every 1h" {
		t.Fatalf("FullSyncCron = %q", cfg.FullSyncCron)
	}
	defaultQuery := (appquery.Config{}).WithDefaults()
	if cfg.Query.MaxResults != defaultQuery.MaxResults {
		t.Fatalf("MaxResults = %d", cfg.Query.MaxResults)
	}
	if cfg.Query.CandidateBudget != defaultQuery.CandidateBudget {
		t.Fatalf("CandidateBudget = %d", cfg.Query.CandidateBudget)
	}
	if cfg.Memory.KeyPadLen != suggestmemory.DefaultKeyPadLen {
		t.Fatalf("KeyPadLen = %d", cfg.Memory.KeyPadLen)
	}
	if cfg.Memory.WildcardKeyCap != suggestmemory.DefaultWildcardKeyCap {
		t.Fatalf("WildcardKeyCap = %d", cfg.Memory.WildcardKeyCap)
	}
}

func TestModuleConfigFromOptionsMapsExplicitValues(t *testing.T) {
	options := apiserveroptions.SuggestOptions{
		Enable:                    true,
		Required:                  true,
		FullSyncCron:              "0 * * * *",
		DeltaSyncCron:             "@every 5m",
		MaxResults:                15,
		InternalMaxResults:        120,
		KeyPadLen:                 30,
		WildcardKeyCap:            80,
		DisableMobileMask:         true,
		LoaderPlaceholderOrgID:    7,
		VisibilityCacheTTLSeconds: 60,
	}
	options.RateLimit.PerOperatorQPS = 5
	options.RateLimit.PerOperatorBurst = 12
	options.RateLimit.MobileKeywordPerOperatorQPS = 2
	options.RateLimit.MobileKeywordPerOperatorBurst = 4
	options.RateLimit.Backend = "redis"
	options.RateLimit.OperatorMapMaxEntries = 1234
	cfg := ModuleConfigFromOptions(options)

	if !cfg.Enable || !cfg.Required {
		t.Fatal("enable/required not mapped")
	}
	if cfg.FullSyncCron != "0 * * * *" || cfg.DeltaSyncCron != "@every 5m" {
		t.Fatalf("cron = %q / %q", cfg.FullSyncCron, cfg.DeltaSyncCron)
	}
	if cfg.Query.MaxResults != 15 || cfg.Query.CandidateBudget != 120 || !cfg.Query.DisableMobileMask {
		t.Fatalf("query = %+v", cfg.Query)
	}
	if cfg.Memory.KeyPadLen != 30 || cfg.Memory.WildcardKeyCap != 80 {
		t.Fatalf("memory = %+v", cfg.Memory)
	}
	if cfg.Loader.PlaceholderOrgID != 7 {
		t.Fatalf("loader org = %d", cfg.Loader.PlaceholderOrgID)
	}
	if cfg.Visibility.CacheTTLSeconds != 60 {
		t.Fatalf("visibility ttl = %d", cfg.Visibility.CacheTTLSeconds)
	}
	if cfg.RateLimit.PerOperatorQPS != 5 ||
		cfg.RateLimit.PerOperatorBurst != 12 ||
		cfg.RateLimit.MobileKeywordPerOperatorQPS != 2 ||
		cfg.RateLimit.MobileKeywordPerOperatorBurst != 4 ||
		cfg.RateLimit.Backend != "redis" ||
		cfg.RateLimit.OperatorMapMaxEntries != 1234 {
		t.Fatalf("rate limit = %+v", cfg.RateLimit)
	}
}

func TestSuggestModuleCheckHealthRequiresSuccessfulRefresh(t *testing.T) {
	module := NewSuggestModule()
	module.config = ModuleConfig{Enable: true}
	module.querier = appquery.DegradedQuerier{}
	if err := module.CheckHealth(); err == nil {
		t.Fatal("CheckHealth() = nil without refresher success")
	}
}

func TestSuggestModuleCheckHealthDisabledPasses(t *testing.T) {
	module := NewSuggestModule()
	module.config = ModuleConfig{Enable: false}
	if err := module.CheckHealth(); err != nil {
		t.Fatalf("CheckHealth() = %v", err)
	}
}
