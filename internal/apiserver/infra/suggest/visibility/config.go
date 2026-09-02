package visibility

import "time"

// Config 控制 visibility 缓存。
type Config struct {
	CacheTTLSeconds int
}

// CacheTTL 返回 TTL duration；<=0 表示关闭缓存。
func (c Config) CacheTTL() time.Duration {
	if c.CacheTTLSeconds <= 0 {
		return 0
	}
	return time.Duration(c.CacheTTLSeconds) * time.Second
}
