package suggest

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// SuggestProfileRequest 档案联想查询入参。
type SuggestProfileRequest struct {
	Principal visibility.Principal
	Keyword   string
	Limit     int
}

// ProfileSuggestItem 应用层返回项（已脱敏手机号）。
type ProfileSuggestItem struct {
	ProfileID   int64
	DisplayName string
	MobileMask  string
	Weight      int
}
