package refreshindex

import "errors"

// ErrRefreshInProgress 刷新互斥锁已被占用。
var ErrRefreshInProgress = errors.New("suggest refresh already in progress")
