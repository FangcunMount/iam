package suggest

import (
	"testing"

	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	cachemodel "github.com/FangcunMount/iam/v3/internal/apiserver/cache"
	suggestratelimit "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/ratelimit"
)

func TestCacheFamilyInspectorsFollowConfiguredRateLimiterBackend(t *testing.T) {
	config := appsuggest.RateLimitConfig{PerOperatorQPS: 1, PerOperatorBurst: 1}

	memoryModule := &SuggestModule{rateLimiter: suggestratelimit.NewMemoryLimiter(config)}
	memoryInspectors := memoryModule.CacheFamilyInspectors()
	if len(memoryInspectors) != 1 || memoryInspectors[0].Descriptor().Family != cachemodel.FamilySuggestMemoryRateLimit {
		t.Fatalf("memory inspectors = %#v", memoryInspectors)
	}

	disabledModule := &SuggestModule{}
	if inspectors := disabledModule.CacheFamilyInspectors(); len(inspectors) != 0 {
		t.Fatalf("disabled inspectors = %#v, want none", inspectors)
	}
}
