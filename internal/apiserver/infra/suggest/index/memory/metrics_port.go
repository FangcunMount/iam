package memory

// IndexMetrics 记录内存索引规模指标。
type IndexMetrics interface {
	SetIndexTerms(n int)
}

type noopIndexMetrics struct{}

func (noopIndexMetrics) SetIndexTerms(int) {}
