package metrics

import "time"

// Recorder 将 application/suggest.SuggestMetrics 端口委托给 Prometheus 指标。
type Recorder struct{}

func (Recorder) RecordQuery(strategy string, resultCount int, mobileShaped bool) {
	RecordQuery(strategy, resultCount, mobileShaped)
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
