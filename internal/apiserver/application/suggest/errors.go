package suggest

import "errors"

var (
	// ErrUnauthenticated 表示缺少有效操作员身份。
	ErrUnauthenticated = errors.New("unauthenticated operating principal")
	// ErrRefreshInProgress indicates that a full or delta refresh already owns
	// the process-local refresh slot.
	ErrRefreshInProgress = errors.New("suggest refresh already in progress")
)
