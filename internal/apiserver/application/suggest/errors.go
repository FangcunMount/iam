package suggest

import (
	"errors"

	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
)

// ErrUnauthenticated 操作员未认证。
var ErrUnauthenticated = errors.New("unauthenticated")

// ErrRefreshInProgress 刷新互斥锁已被占用。
var ErrRefreshInProgress = apprefresh.ErrRefreshInProgress
