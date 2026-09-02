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
	calls      int
}

func (r *stubRecaller) Recall(context.Context, appquery.RecallRequest) ([]domainsearch.Candidate, error) {
	r.calls++
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
