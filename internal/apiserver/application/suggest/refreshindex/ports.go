package refreshindex

import (
	"context"
	"time"

	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

// ProjectionSource 提供索引刷新所需的 Full/Delta 数据。
type ProjectionSource interface {
	Full(ctx context.Context) ([]domainprofile.SuggestibleProfile, error)
	Delta(ctx context.Context, since time.Time) ([]ProjectionChange, error)
}

// IndexWriter 写入物理索引。
type IndexWriter interface {
	Replace(ctx context.Context, profiles []domainprofile.SuggestibleProfile) error
	Apply(ctx context.Context, changes []ProjectionChange) error
}

// Metrics 刷新用例指标。
type Metrics interface {
	ObserveRefresh(kind string, seconds float64)
	RecordRefresh(kind, result string, upserts, tombstones int, completedAt time.Time)
}

type noopMetrics struct{}

func (noopMetrics) ObserveRefresh(string, float64)                    {}
func (noopMetrics) RecordRefresh(string, string, int, int, time.Time) {}
