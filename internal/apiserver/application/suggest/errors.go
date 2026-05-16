package suggest

import "errors"

var (
	// ErrUnauthenticated 表示缺少有效操作员身份。
	ErrUnauthenticated = errors.New("unauthenticated operating principal")
)
