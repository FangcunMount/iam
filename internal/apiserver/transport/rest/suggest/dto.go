package suggest

import (
	"strconv"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
)

// ProfileSuggestResponseItem REST 返回项（不包含明文手机号）。
type ProfileSuggestResponseItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MobileMask string `json:"mobile_mask,omitempty"`
	Weight     int    `json:"weight"`
}

// ProfileSuggestResponse 描述档案联想接口的统一成功响应。
// 运行时仍由 core.BaseHandler 生成相同的 JSON envelope。
type ProfileSuggestResponse struct {
	Code    int                          `json:"code" binding:"required"`
	Message string                       `json:"message" binding:"required"`
	Data    []ProfileSuggestResponseItem `json:"data" binding:"required"`
}

func toProfileSuggestResponseItems(items []appquery.ResultItem) []ProfileSuggestResponseItem {
	out := make([]ProfileSuggestResponseItem, 0, len(items))
	for _, it := range items {
		out = append(out, ProfileSuggestResponseItem{
			ID:         strconv.FormatInt(it.ProfileID, 10),
			Name:       it.DisplayName,
			MobileMask: it.MobileMask,
			Weight:     it.Weight,
		})
	}
	return out
}
