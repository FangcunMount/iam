package queryprofile_test

import (
	"context"
	"errors"
	"testing"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

type stubScope struct {
	scope visibility.Scope
	err   error
	calls int
}

func (s *stubScope) ResolveScope(context.Context, visibility.Principal) (visibility.Scope, error) {
	s.calls++
	if s.err != nil {
		return visibility.Scope{}, s.err
	}
	return s.scope, nil
}

type stubRecaller struct {
	candidates []domainsearch.Candidate
	err        error
	calls      int
	lastReq    appquery.RecallRequest
}

func (r *stubRecaller) Recall(_ context.Context, req appquery.RecallRequest) ([]domainsearch.Candidate, error) {
	r.calls++
	r.lastReq = req
	if r.err != nil {
		return nil, r.err
	}
	return r.candidates, nil
}

type stubMetrics struct {
	kind         domainsearch.DecisionKind
	resultCount  int
	mobileShaped bool
	matched      int
	visible      int
}

func (m *stubMetrics) RecordQuery(kind domainsearch.DecisionKind, resultCount int, mobileShaped bool) {
	m.kind = kind
	m.resultCount = resultCount
	m.mobileShaped = mobileShaped
}
func (m *stubMetrics) ObserveSelection(matched, visible int) {
	m.matched = matched
	m.visible = visible
}

func TestQueryProfileEmptyKeywordSkipsScopeAndRecaller(t *testing.T) {
	scope := &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}
	recaller := &stubRecaller{}
	svc := appquery.NewService(appquery.Config{MaxResults: 20, CandidateBudget: 200}, scope, recaller, nil)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "  ",
	})
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if scope.calls != 0 || recaller.calls != 0 {
		t.Fatalf("scope=%d recaller=%d", scope.calls, recaller.calls)
	}
}

func TestQueryProfileUnauthenticated(t *testing.T) {
	svc := appquery.NewService(appquery.Config{}, &stubScope{}, &stubRecaller{}, nil)
	_, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 0},
		Keyword:   "a",
	})
	if !errors.Is(err, appquery.ErrUnauthenticated) {
		t.Fatalf("err=%v", err)
	}
}

func TestQueryProfileNilDepsReturnsEmpty(t *testing.T) {
	svc := appquery.NewService(appquery.Config{}, nil, nil, nil)
	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "a",
	})
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestQueryProfileMobileDeniedSkipsRecaller(t *testing.T) {
	scope := &stubScope{scope: visibility.NewScope(true, false, 0, nil, nil)}
	recaller := &stubRecaller{candidates: []domainsearch.Candidate{{Profile: profile.New(1, "a", nil, 1, 0, nil), Strength: domainsearch.MatchExact}}}
	metrics := &stubMetrics{}
	svc := appquery.NewService(appquery.Config{}, scope, recaller, metrics)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "13800138000",
	})
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if recaller.calls != 0 {
		t.Fatal("recaller should not run")
	}
	if metrics.kind != domainsearch.DecisionDenied || !metrics.mobileShaped {
		t.Fatalf("metrics=%+v", metrics)
	}
}

func TestQueryProfileScopeErrorPropagates(t *testing.T) {
	want := errors.New("scope failed")
	scope := &stubScope{err: want}
	svc := appquery.NewService(appquery.Config{}, scope, &stubRecaller{}, nil)
	_, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "a",
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestQueryProfileRecallErrorPropagates(t *testing.T) {
	want := errors.New("recall failed")
	svc := appquery.NewService(appquery.Config{}, &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}, &stubRecaller{err: want}, nil)
	_, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "a",
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestQueryProfileHappyPathPrefix(t *testing.T) {
	candidates := []domainsearch.Candidate{
		{Profile: profile.New(3, "张三丰", nil, 8, 0, nil), Strength: domainsearch.MatchDirectPrefix},
		{Profile: profile.New(1, "张三", nil, 5, 0, nil), Strength: domainsearch.MatchDirectPrefix},
	}
	scope := &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}
	metrics := &stubMetrics{}
	svc := appquery.NewService(appquery.Config{MaxResults: 20}, scope, &stubRecaller{candidates: candidates}, metrics)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "张",
		Limit:     5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProfileID != 3 {
		t.Fatalf("got=%#v", got)
	}
	if metrics.kind != domainsearch.DecisionPrefixText || metrics.resultCount != 2 {
		t.Fatalf("metrics=%+v", metrics)
	}
	if metrics.matched != 2 || metrics.visible != 2 {
		t.Fatalf("selection metrics=%+v", metrics)
	}
}

func TestQueryProfileLimitClamp(t *testing.T) {
	candidates := make([]domainsearch.Candidate, 0, 30)
	for i := int64(1); i <= 30; i++ {
		candidates = append(candidates, domainsearch.Candidate{
			Profile:  profile.New(i, "张", nil, int(i), 0, nil),
			Strength: domainsearch.MatchDirectPrefix,
		})
	}
	svc := appquery.NewService(appquery.Config{MaxResults: 10}, &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}, &stubRecaller{candidates: candidates}, nil)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "张",
		Limit:     999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("got len=%d, want 10", len(got))
	}
}

func TestQueryProfileScopeFiltersResults(t *testing.T) {
	candidates := []domainsearch.Candidate{
		{Profile: profile.New(1, "张三", nil, 5, 1, nil), Strength: domainsearch.MatchDirectPrefix},
		{Profile: profile.New(2, "张磊", nil, 5, 2, nil), Strength: domainsearch.MatchDirectPrefix},
	}
	scope := &stubScope{scope: visibility.NewScope(false, false, 0, []int64{1}, nil)}
	metrics := &stubMetrics{}
	svc := appquery.NewService(appquery.Config{}, scope, &stubRecaller{candidates: candidates}, metrics)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "张",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProfileID != 1 {
		t.Fatalf("got=%#v", got)
	}
	if metrics.matched != 2 || metrics.visible != 1 {
		t.Fatalf("selection metrics=%+v", metrics)
	}
}

func TestQueryProfileDefaultMobileMask(t *testing.T) {
	candidates := []domainsearch.Candidate{
		{Profile: profile.New(1, "张三", []string{"13800138000"}, 5, 0, nil), Strength: domainsearch.MatchExact},
	}
	svc := appquery.NewService(appquery.Config{}, &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}, &stubRecaller{candidates: candidates}, nil)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "13800138000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MobileMask != "138****8000" {
		t.Fatalf("got=%#v", got)
	}
}

func TestQueryProfileDisableMobileMask(t *testing.T) {
	candidates := []domainsearch.Candidate{
		{Profile: profile.New(1, "张三", []string{"13800138000"}, 5, 0, nil), Strength: domainsearch.MatchExact},
	}
	svc := appquery.NewService(appquery.Config{DisableMobileMask: true}, &stubScope{scope: visibility.NewScope(true, true, 0, nil, nil)}, &stubRecaller{candidates: candidates}, nil)

	got, err := svc.QueryProfile(context.Background(), appquery.Command{
		Principal: visibility.Principal{OperatorID: 1},
		Keyword:   "13800138000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MobileMask != "13800138000" {
		t.Fatalf("got=%#v", got)
	}
}

func TestDecisionKindLabel(t *testing.T) {
	if appquery.DecisionKindLabel(domainsearch.DecisionDenied) != "mobile_denied" {
		t.Fatal("denied label")
	}
	if appquery.DecisionKindLabel(domainsearch.DecisionNumericExact) != "numeric_exact" {
		t.Fatal("numeric label")
	}
	if appquery.DecisionKindLabel(domainsearch.DecisionPrefixText) != "prefix_text" {
		t.Fatal("prefix label")
	}
}
