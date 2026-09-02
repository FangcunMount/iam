package suggest

import (
	"context"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
)

// profileSuggestorAdapter 将 queryprofile.Querier 适配为 ProfileSuggestor。
type profileSuggestorAdapter struct {
	inner appquery.Querier
}

// NewProfileSuggestor 包装 queryprofile.Querier 供 transport 使用。
func NewProfileSuggestor(q appquery.Querier) ProfileSuggestor {
	if q == nil {
		return profileSuggestorAdapter{}
	}
	return profileSuggestorAdapter{inner: q}
}

func (a profileSuggestorAdapter) SuggestProfile(ctx context.Context, req SuggestProfileRequest) ([]ProfileSuggestItem, error) {
	if a.inner == nil {
		return []ProfileSuggestItem{}, nil
	}
	items, err := a.inner.QueryProfile(ctx, appquery.Command{
		Principal: req.Principal,
		Keyword:   req.Keyword,
		Limit:     req.Limit,
	})
	if err != nil {
		if err == appquery.ErrUnauthenticated {
			return nil, ErrUnauthenticated
		}
		return nil, err
	}
	out := make([]ProfileSuggestItem, 0, len(items))
	for _, item := range items {
		out = append(out, ProfileSuggestItem{
			ProfileID:   item.ProfileID,
			DisplayName: item.DisplayName,
			MobileMask:  item.MobileMask,
			Weight:      item.Weight,
		})
	}
	return out, nil
}

// DegradedService 索引未就绪时返回空结果。
var DegradedService ProfileSuggestor = NewProfileSuggestor(appquery.DegradedQuerier{})
