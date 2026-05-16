package suggest

// SuggestMetrics 记录 suggest 可观测性信号（由 infra 层实现）。
type SuggestMetrics interface {
	RecordQuery(strategy string, resultCount int, mobileShaped bool)
	ObserveRefresh(kind string, seconds float64)
	RecordRateLimited(mobileKeyword bool)
}

type noopSuggestMetrics struct{}

func (noopSuggestMetrics) RecordQuery(string, int, bool)         {}
func (noopSuggestMetrics) ObserveRefresh(string, float64)       {}
func (noopSuggestMetrics) RecordRateLimited(bool)               {}
