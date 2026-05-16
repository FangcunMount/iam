package suggest

import "context"

// DegradedService suggest 不可用时的空实现，避免拖垮核心能力。
type DegradedService struct{}

// SuggestProfile implements ProfileSuggestor.
func (DegradedService) SuggestProfile(context.Context, SuggestProfileRequest) ([]ProfileSuggestItem, error) {
	return []ProfileSuggestItem{}, nil
}
