package suggest

import "testing"

func TestSearchPolicyDecide(t *testing.T) {
	policy := SearchPolicy{}
	scopeNoMobile := ProfileAccessScope{AllowMobileSearch: false}
	scopeMobileOK := ProfileAccessScope{AllowMobileSearch: true, AllProfile: true}

	kwMobile := NewKeyword("13812345678")
	kwShort := NewKeyword("42")
	kwText := NewKeyword("张")
	kwEmpty := NewKeyword("  ")

	if d := policy.Decide(kwMobile, scopeNoMobile); d.Allowed() || d.MetricName() != "mobile_denied" {
		t.Fatalf("mobile denied = %+v", d)
	}
	if d := policy.Decide(kwMobile, scopeMobileOK); !d.Allowed() || d.MetricName() != "numeric_exact" || d.Mode() != SearchModeExact {
		t.Fatalf("mobile allowed = %+v", d)
	}
	if d := policy.Decide(kwShort, scopeNoMobile); !d.Allowed() || d.MetricName() != "numeric_exact" {
		t.Fatalf("short numeric = %+v", d)
	}
	if d := policy.Decide(kwText, scopeNoMobile); !d.Allowed() || d.MetricName() != "prefix_text" || d.Mode() != SearchModePrefix {
		t.Fatalf("text = %+v", d)
	}
	if d := policy.Decide(kwEmpty, scopeNoMobile); d.Allowed() {
		t.Fatalf("empty = %+v", d)
	}
}

func TestKeywordKindAndIsMobileShaped(t *testing.T) {
	if NewKeyword("").Kind() != KeywordKindEmpty {
		t.Fatal("empty kind")
	}
	if NewKeyword("张").Kind() != KeywordKindText {
		t.Fatal("text kind")
	}
	if NewKeyword("42").Kind() != KeywordKindNumeric || NewKeyword("42").IsMobileShaped() {
		t.Fatal("short numeric")
	}
	if !NewKeyword("13812345678").IsMobileShaped() {
		t.Fatal("mobile shaped")
	}
}
