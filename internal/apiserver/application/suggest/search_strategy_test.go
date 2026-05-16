package suggest

import (
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestSelectProfileSearchStrategyDefaults(t *testing.T) {
	def := DefaultProfileSearchStrategies()
	kwMobile := domainsuggest.NewKeyword("13812345678")
	kwShort := domainsuggest.NewKeyword("42")
	kwText := domainsuggest.NewKeyword("张")

	scopeNoMobile := domainsuggest.ProfileAccessScope{AllowMobileSearch: false}
	scopeMobileOK := domainsuggest.ProfileAccessScope{AllowMobileSearch: true}

	if s := selectProfileSearchStrategy(def, kwMobile, scopeNoMobile); s == nil || s.Name() != "mobile_denied" {
		t.Fatalf("want mobile_denied, got %v", s)
	}
	if s := selectProfileSearchStrategy(def, kwMobile, scopeMobileOK); s == nil || s.Name() != "numeric_exact" {
		t.Fatalf("want numeric_exact for allowed mobile, got %v", s)
	}
	if s := selectProfileSearchStrategy(def, kwShort, scopeNoMobile); s == nil || s.Name() != "numeric_exact" {
		t.Fatalf("want numeric_exact, got %v", s)
	}
	if s := selectProfileSearchStrategy(def, kwText, scopeNoMobile); s == nil || s.Name() != "prefix_text" {
		t.Fatalf("want prefix_text, got %v", s)
	}
}

func TestNewServiceWithRuntimeStrategiesFiltersNil(t *testing.T) {
	svc := NewServiceWithRuntimeStrategies(Config{}, nil, nil, []ProfileSearchStrategy{nil, mobileDeniedStrategy{}}, nil)
	if len(svc.strategies) != 1 {
		t.Fatalf("strategies len = %d", len(svc.strategies))
	}
}

func TestNewServiceWithRuntimeStrategiesEmptyFallsBackToDefault(t *testing.T) {
	svc := NewServiceWithRuntimeStrategies(Config{}, nil, nil, []ProfileSearchStrategy{}, nil)
	if len(svc.strategies) < 3 {
		t.Fatalf("expected default chain, got len %d", len(svc.strategies))
	}
}
