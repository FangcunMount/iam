package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

func TestDecisionKindLabel(t *testing.T) {
	if DecisionKindLabel(domainsearch.DecisionDenied) != "mobile_denied" {
		t.Fatal("denied label")
	}
	if DecisionKindLabel(domainsearch.DecisionNumericExact) != "numeric_exact" {
		t.Fatal("numeric label")
	}
	if DecisionKindLabel(domainsearch.DecisionPrefixText) != "prefix_text" {
		t.Fatal("prefix label")
	}
}

func TestRecorderMapsDecisionKindToStrategyLabel(t *testing.T) {
	rec := Recorder{}
	beforeDenied := testutil.ToFloat64(queriesTotal.WithLabelValues("mobile_denied"))
	rec.RecordQuery(domainsearch.DecisionDenied, 0, true)
	if testutil.ToFloat64(queriesTotal.WithLabelValues("mobile_denied")) <= beforeDenied {
		t.Fatal("mobile_denied counter not incremented")
	}

	beforeNumeric := testutil.ToFloat64(queriesTotal.WithLabelValues("numeric_exact"))
	rec.RecordQuery(domainsearch.DecisionNumericExact, 1, false)
	if testutil.ToFloat64(queriesTotal.WithLabelValues("numeric_exact")) <= beforeNumeric {
		t.Fatal("numeric_exact counter not incremented")
	}

	beforePrefix := testutil.ToFloat64(queriesTotal.WithLabelValues("prefix_text"))
	rec.RecordQuery(domainsearch.DecisionPrefixText, 2, false)
	if testutil.ToFloat64(queriesTotal.WithLabelValues("prefix_text")) <= beforePrefix {
		t.Fatal("prefix_text counter not incremented")
	}
}

func TestRecorderObserveSelectionAndIndexTerms(t *testing.T) {
	rec := Recorder{}
	rec.ObserveSelection(10, 4)
	rec.SetIndexTerms(42)
	if got := testutil.ToFloat64(indexTerms); got != 42 {
		t.Fatalf("index_terms = %v, want 42", got)
	}
}

func TestRecorderRefreshAndRateLimitMetrics(t *testing.T) {
	rec := Recorder{}
	rec.RecordRateLimited(true)
	rec.RecordRateLimited(false)
	rec.ObserveRefresh("full", 0.5)
	rec.RecordRefresh("full", "success", 3, 1, time.Unix(100, 0))
	rec.RecordRefresh("delta", "refresh_in_progress", 0, 0, time.Time{})
	rec.RecordRefresh("delta", "failed", 0, 0, time.Time{})

	assertCounterVecLabels(t, refreshTotal, prometheus.Labels{"kind": "full", "result": "success"})
	assertCounterVecLabels(t, refreshTotal, prometheus.Labels{"kind": "delta", "result": "refresh_in_progress"})
	assertCounterVecLabels(t, refreshTotal, prometheus.Labels{"kind": "delta", "result": "failed"})
	assertCounterVecLabels(t, rateLimitedTotal, prometheus.Labels{"kind": "mobile"})
	assertCounterVecLabels(t, rateLimitedTotal, prometheus.Labels{"kind": "std"})
}

func assertCounterVecLabels(t *testing.T, vec *prometheus.CounterVec, labels prometheus.Labels) {
	t.Helper()
	m, err := vec.GetMetricWith(labels)
	if err != nil {
		t.Fatalf("metric with %#v: %v", labels, err)
	}
	if testutil.ToFloat64(m) <= 0 {
		t.Fatalf("metric %#v not incremented", labels)
	}
}

func TestMetricFullNames(t *testing.T) {
	names := []string{
		"iam_suggest_queries_total",
		"iam_suggest_mobile_shaped_queries_total",
		"iam_suggest_results_returned",
		"iam_suggest_matched_candidates",
		"iam_suggest_visible_after_scope",
		"iam_suggest_index_terms",
		"iam_suggest_rate_limited_total",
		"iam_suggest_refresh_duration_seconds",
		"iam_suggest_refresh_total",
		"iam_suggest_refresh_items_total",
		"iam_suggest_last_success_timestamp_seconds",
	}
	for _, name := range names {
		if !strings.Contains(metricRegistryDump(), name) {
			t.Fatalf("missing metric %q", name)
		}
	}
}

func metricRegistryDump() string {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, mf := range mfs {
		if mf.GetName() == "" || !strings.HasPrefix(mf.GetName(), "iam_suggest_") {
			continue
		}
		b.WriteString(mf.GetName())
		b.WriteByte('\n')
	}
	return b.String()
}
