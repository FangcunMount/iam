package suggest

import (
	"context"
	"errors"
	"testing"
	"time"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

type recordingScopeProvider struct {
	scope domainsuggest.ProfileAccessScope
	err   error
	calls int
}

func (p *recordingScopeProvider) ResolveProfileAccessScope(context.Context, domainsuggest.OperatingPrincipal) (domainsuggest.ProfileAccessScope, error) {
	p.calls++
	if p.err != nil {
		return domainsuggest.ProfileAccessScope{}, p.err
	}
	return p.scope, nil
}

type recordingIndex struct {
	terms []domainsuggest.ProfileSearchTerm
	calls int
}

func (i *recordingIndex) SuggestProfile(domainsuggest.Query, domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	i.calls++
	return append([]domainsuggest.ProfileSearchTerm(nil), i.terms...)
}

type recordingRuntime struct {
	index   *recordingIndex
	current ProfileSuggestionIndex
	calls   int
}

func (r *recordingRuntime) Current() ProfileSuggestionIndex {
	r.calls++
	if r.index != nil {
		return r.index
	}
	return r.current
}

func (r *recordingRuntime) Replace([]domainsuggest.ProfileSearchTerm) ProfileSuggestionIndex {
	return nil
}
func (r *recordingRuntime) ApplyDelta([]domainsuggest.ProfileIndexMutation) error { return nil }

type recordingMetrics struct {
	strategy     string
	resultCount  int
	mobileShaped bool
	calls        int
}

func (m *recordingMetrics) RecordQuery(strategy string, resultCount int, mobileShaped bool) {
	m.calls++
	m.strategy = strategy
	m.resultCount = resultCount
	m.mobileShaped = mobileShaped
}

func (m *recordingMetrics) ObserveRefresh(string, float64)                    {}
func (m *recordingMetrics) RecordRefresh(string, string, int, int, time.Time) {}
func (m *recordingMetrics) RecordRateLimited(bool)                            {}

func newServiceForCharacterization(scope ProfileAccessScopeProvider, index *recordingIndex, metrics SuggestMetrics) *Service {
	runtime := &recordingRuntime{index: index}
	cfg := Config{MaxResults: 20, InternalMaxResults: 200, KeyPadLen: 25}
	if metrics == nil {
		metrics = noopSuggestMetrics{}
	}
	return NewServiceWithRuntime(cfg, runtime, scope, metrics)
}

func TestServiceSuggestProfileUnauthenticatedDoesNotCallScopeOrIndex(t *testing.T) {
	scope := &recordingScopeProvider{}
	index := &recordingIndex{}
	metrics := &recordingMetrics{}
	svc := newServiceForCharacterization(scope, index, metrics)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 0},
		Keyword:   "张",
	})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if got != nil {
		t.Fatalf("got = %#v, want nil", got)
	}
	if scope.calls != 0 || index.calls != 0 {
		t.Fatalf("scope calls = %d, index calls = %d, want 0/0", scope.calls, index.calls)
	}
}

func TestServiceSuggestProfileEmptyKeywordDoesNotCallScopeOrIndex(t *testing.T) {
	scope := &recordingScopeProvider{}
	index := &recordingIndex{}
	metrics := &recordingMetrics{}
	svc := newServiceForCharacterization(scope, index, metrics)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "  ",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %#v, want empty", got)
	}
	if scope.calls != 0 || index.calls != 0 {
		t.Fatalf("scope calls = %d, index calls = %d, want 0/0", scope.calls, index.calls)
	}
}

func TestServiceSuggestProfileScopeErrorPropagates(t *testing.T) {
	wantErr := errors.New("scope failed")
	scope := &recordingScopeProvider{err: wantErr}
	index := &recordingIndex{}
	svc := newServiceForCharacterization(scope, index, nil)

	_, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "张",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if index.calls != 0 {
		t.Fatalf("index calls = %d, want 0", index.calls)
	}
}

func TestServiceSuggestProfileNilScopeProviderReturnsEmpty(t *testing.T) {
	svc := NewServiceWithRuntime(Config{}, &recordingRuntime{index: &recordingIndex{}}, nil, nil)
	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1},
		Keyword:   "张",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %#v, want empty", got)
	}
}

func TestServiceSuggestProfileNoCurrentIndexReturnsNonemptyEmptySlice(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true}}
	svc := NewServiceWithRuntime(Config{}, &recordingRuntime{}, scope, nil)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "张",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %#v, want non-nil empty slice", got)
	}
}

func TestServiceSuggestProfileMobileDeniedDoesNotCallIndex(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllowMobileSearch: false}}
	index := &recordingIndex{
		terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13812345678"}, 5, 0, nil),
		},
	}
	metrics := &recordingMetrics{}
	svc := newServiceForCharacterization(scope, index, metrics)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "13812345678",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %#v, want empty", got)
	}
	if scope.calls != 1 {
		t.Fatalf("scope calls = %d, want 1", scope.calls)
	}
	if index.calls != 0 {
		t.Fatalf("index calls = %d, want 0", index.calls)
	}
	if metrics.strategy != "mobile_denied" || !metrics.mobileShaped {
		t.Fatalf("metrics = (%q, mobile=%v), want (mobile_denied, true)", metrics.strategy, metrics.mobileShaped)
	}
}

func TestServiceSuggestProfileNumericExactCallsIndex(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true}}
	index := &recordingIndex{
		terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(42, "档案42", nil, 5, 0, nil),
		},
	}
	metrics := &recordingMetrics{}
	svc := newServiceForCharacterization(scope, index, metrics)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "42",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 || got[0].ProfileID != 42 {
		t.Fatalf("got = %#v", got)
	}
	if index.calls != 1 {
		t.Fatalf("index calls = %d, want 1", index.calls)
	}
	if metrics.strategy != "numeric_exact" {
		t.Fatalf("strategy = %q, want numeric_exact", metrics.strategy)
	}
}

func TestServiceSuggestProfilePrefixTextCallsIndex(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true}}
	index := &recordingIndex{
		terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", nil, 5, 0, nil),
		},
	}
	metrics := &recordingMetrics{}
	svc := newServiceForCharacterization(scope, index, metrics)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "张",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got = %#v", got)
	}
	if metrics.strategy != "prefix_text" {
		t.Fatalf("strategy = %q, want prefix_text", metrics.strategy)
	}
}

func TestServiceSuggestProfileLimitClamp(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true}}
	index := &recordingIndex{terms: []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "a", nil, 1, 0, nil),
	}}
	cfg := Config{MaxResults: 10, InternalMaxResults: 100, KeyPadLen: 25}
	svc := NewServiceWithRuntime(cfg, &recordingRuntime{index: index}, scope, nil)

	cases := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"zero uses max", 0, 10},
		{"negative uses max", -1, 10},
		{"over max clamped", 99, 10},
		{"within max kept", 5, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index.calls = 0
			_, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
				Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
				Keyword:   "a",
				Limit:     tc.limit,
			})
			if err != nil {
				t.Fatal(err)
			}
			if index.calls != 1 {
				t.Fatalf("index calls = %d", index.calls)
			}
		})
	}
}

func TestServiceSuggestProfileDefaultMobileMask(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true, AllowMobileSearch: true}}
	index := &recordingIndex{
		terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, nil),
		},
	}
	svc := newServiceForCharacterization(scope, index, nil)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "13800138000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MobileMask != "138****8000" {
		t.Fatalf("MobileMask = %q, want 138****8000", got[0].MobileMask)
	}
}

func TestServiceSuggestProfileDisableMobileMaskReturnsPlainMobile(t *testing.T) {
	scope := &recordingScopeProvider{scope: domainsuggest.ProfileAccessScope{AllProfile: true, AllowMobileSearch: true}}
	index := &recordingIndex{
		terms: []domainsuggest.ProfileSearchTerm{
			domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, nil),
		},
	}
	cfg := Config{MaxResults: 20, InternalMaxResults: 200, KeyPadLen: 25, DisableMobileMask: true}
	svc := NewServiceWithRuntime(cfg, &recordingRuntime{index: index}, scope, nil)

	got, err := svc.SuggestProfile(context.Background(), SuggestProfileRequest{
		Principal: domainsuggest.OperatingPrincipal{OperatorID: 1, TenantDomain: "fangcun"},
		Keyword:   "13800138000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MobileMask != "13800138000" {
		t.Fatalf("MobileMask = %q, want plaintext", got[0].MobileMask)
	}
}
