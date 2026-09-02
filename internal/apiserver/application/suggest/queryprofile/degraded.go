package queryprofile

import "context"

// DegradedQuerier 索引未就绪时返回空结果。
type DegradedQuerier struct{}

func (DegradedQuerier) QueryProfile(context.Context, Command) ([]ResultItem, error) {
	return []ResultItem{}, nil
}

var _ Querier = DegradedQuerier{}
