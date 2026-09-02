package suggest

import (
	"context"
	"testing"

	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	suggestmemory "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/index/memory"
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
	cfg := ModuleConfigFromOptions(apiserveroptions.SuggestOptions{
		Enable:                     true,
		Required:                   true,
		FullSyncCron:               "0 * * * *",
		DeltaSyncCron:              "@every 5m",
		MaxResults:                 15,
		InternalMaxResults:         120,
		KeyPadLen:                  30,
		WildcardKeyCap:             80,
		DisableMobileMask:          true,
		LoaderPlaceholderOrgID:     7,
		VisibilityCacheTTLSeconds:  60,
	})

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
}

func TestSuggestModuleCheckHealthRequiresSuccessfulRefresh(t *testing.T) {
	module := NewSuggestModule()
	module.config = ModuleConfig{Enable: true}
	module.service = degradedSuggestor{}
	if err := module.CheckHealth(); err == nil {
		t.Fatal("CheckHealth() = nil without refresher success")
	}
}

type degradedSuggestor struct{}

func (degradedSuggestor) SuggestProfile(context.Context, appsuggest.SuggestProfileRequest) ([]appsuggest.ProfileSuggestItem, error) {
	return nil, nil
}

func TestSuggestModuleCheckHealthDisabledPasses(t *testing.T) {
	module := NewSuggestModule()
	module.config = ModuleConfig{Enable: false}
	if err := module.CheckHealth(); err != nil {
		t.Fatalf("CheckHealth() = %v", err)
	}
}
