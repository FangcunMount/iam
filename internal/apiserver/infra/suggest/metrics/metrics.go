package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	queriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "queries_total",
			Help:      "档案 suggest 查询次数，按策略标签。",
		},
		[]string{"strategy"},
	)
	mobileQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "mobile_shaped_queries_total",
			Help:      "手机号形态的数字关键词请求次数（不计是否授权）。",
		},
	)
	resultsReturned = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "results_returned",
			Help:      "单次查询返回条数（scope 过滤并排序后）。",
			Buckets:   []float64{0, 1, 2, 3, 5, 10, 15, 20, 30, 50},
		},
	)
	matchedCandidates = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "matched_candidates",
			Help:      "索引召回的 profileID 数（scope 过滤前）。",
			Buckets:   []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500},
		},
	)
	visibleAfterScope = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "visible_after_scope",
			Help:      "数据权限过滤后的候选数（排序截断前）。",
			Buckets:   []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500},
		},
	)
	indexTerms = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "index_terms",
			Help:      "当前内存索引中的档案条数。",
		},
	)
	refreshDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "iam",
			Subsystem: "suggest",
			Name:      "refresh_duration_seconds",
			Help:      "索引全量/增量刷新耗时（秒）。",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"kind"},
	)
)

// RecordQuery 记录一次完成的 suggest 查询（service 层，策略已选定）。
func RecordQuery(strategy string, resultCount int, mobileShaped bool) {
	if strategy == "" {
		strategy = "unknown"
	}
	queriesTotal.WithLabelValues(strategy).Inc()
	if mobileShaped {
		mobileQueriesTotal.Inc()
	}
	resultsReturned.Observe(float64(resultCount))
}

// ObserveIndexFilter 记录召回与 scope 过滤后的规模（store 层）。
func ObserveIndexFilter(matched, visible int) {
	if matched < 0 {
		matched = 0
	}
	if visible < 0 {
		visible = 0
	}
	matchedCandidates.Observe(float64(matched))
	visibleAfterScope.Observe(float64(visible))
}

// SetIndexTerms 更新当前索引档案条数。
func SetIndexTerms(n int) {
	if n < 0 {
		n = 0
	}
	indexTerms.Set(float64(n))
}

// ObserveRefresh 记录索引刷新耗时（秒）。
func ObserveRefresh(kind string, seconds float64) {
	if kind == "" {
		kind = "unknown"
	}
	if seconds < 0 {
		seconds = 0
	}
	refreshDuration.WithLabelValues(kind).Observe(seconds)
}
