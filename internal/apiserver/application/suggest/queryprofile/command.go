package queryprofile

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// Command 档案联想查询入参。
type Command struct {
	Principal visibility.Principal
	Keyword   string
	Limit     int
}

// ResultItem 应用层返回项。
type ResultItem struct {
	ProfileID   int64
	DisplayName string
	MobileMask  string
	Weight      int
}
