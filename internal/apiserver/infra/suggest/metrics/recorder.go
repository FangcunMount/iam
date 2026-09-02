package metrics

import (
	"time"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

// RateLimitRecorder 记录 REST 限流指标。
type RateLimitRecorder interface {
	RecordRateLimited(mobileKeyword bool)
}

// Recorder 实现 query、refresh 与 rate-limit 指标端口。
type Recorder struct{}

func (Recorder) RecordQuery(kind domainsearch.DecisionKind, resultCount int, mobileShaped bool) {
	RecordQuery(DecisionKindLabel(kind), resultCount, mobileShaped)
}

// DecisionKindLabel 将 DecisionKind 映射为 Prometheus 策略标签。
func DecisionKindLabel(kind domainsearch.DecisionKind) string {
	switch kind {
	case domainsearch.DecisionDenied:
		return "mobile_denied"
	case domainsearch.DecisionNumericExact:
		return "numeric_exact"
	default:
		return "prefix_text"
	}
}

func (Recorder) ObserveSelection(matched, visible int) {
	ObserveIndexFilter(matched, visible)
}

func (Recorder) ObserveRefresh(kind string, seconds float64) {
	ObserveRefresh(kind, seconds)
}

func (Recorder) RecordRefresh(kind, result string, upserts, tombstones int, completedAt time.Time) {
	RecordRefresh(kind, result, upserts, tombstones, completedAt)
}

func (Recorder) RecordRateLimited(mobileKeyword bool) {
	RecordRateLimited(mobileKeyword)
}

// SetIndexTerms 实现 memory.IndexMetrics。
func (Recorder) SetIndexTerms(n int) {
	SetIndexTerms(n)
}

var (
	_ appquery.Metrics   = Recorder{}
	_ apprefresh.Metrics = Recorder{}
	_ RateLimitRecorder  = Recorder{}
)
