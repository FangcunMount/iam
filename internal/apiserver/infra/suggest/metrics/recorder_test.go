package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

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
}
