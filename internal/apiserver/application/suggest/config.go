package suggest

import (
	"math"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// RateLimitConfig REST 层按操作员限流；PerOperatorQPS<=0 表示关闭。
type RateLimitConfig struct {
	PerOperatorQPS                float64 `json:"per_operator_qps" mapstructure:"per_operator_qps"`
	PerOperatorBurst              int     `json:"per_operator_burst" mapstructure:"per_operator_burst"`
	MobileKeywordPerOperatorQPS   float64 `json:"mobile_keyword_per_operator_qps" mapstructure:"mobile_keyword_per_operator_qps"`
	MobileKeywordPerOperatorBurst int     `json:"mobile_keyword_per_operator_burst" mapstructure:"mobile_keyword_per_operator_burst"`
	// Backend memory（默认）| redis；redis 需 REST 注入 Redis 客户端。
	Backend string `json:"backend" mapstructure:"backend"`
	// OperatorMapMaxEntries memory 后端 operator→limiter 映射上限，超限 LRU 淘汰。
	OperatorMapMaxEntries int `json:"operator_map_max_entries" mapstructure:"operator_map_max_entries"`
}

func (r RateLimitConfig) withDefaults() RateLimitConfig {
	if r.PerOperatorQPS <= 0 {
		return RateLimitConfig{}
	}
	out := r
	if out.PerOperatorBurst <= 0 {
		out.PerOperatorBurst = max(5, int(math.Ceil(out.PerOperatorQPS*2)))
	}
	if out.MobileKeywordPerOperatorQPS <= 0 {
		out.MobileKeywordPerOperatorQPS = out.PerOperatorQPS
	}
	if out.MobileKeywordPerOperatorBurst <= 0 {
		out.MobileKeywordPerOperatorBurst = max(3, int(math.Ceil(out.MobileKeywordPerOperatorQPS*2)))
	}
	if out.OperatorMapMaxEntries <= 0 {
		out.OperatorMapMaxEntries = 10000
	}
	if out.Backend == "" {
		out.Backend = "memory"
	}
	return out
}

// Config 控制 suggest 模块行为
type Config struct {
	Enable             bool
	Required           bool
	FullSyncCron       string
	DeltaSyncCron      string
	MaxResults         int
	InternalMaxResults int
	KeyPadLen          int
	FullSQL            string
	DeltaSQL           string
	// DisableMobileMask 为 true 时返回明文手机号（仅特殊排障；默认应关闭）。
	DisableMobileMask bool
	// LoaderPlaceholderOrgID 注入内建 Loader SQL 的 org_id 占位；0=不虚构组织维度。
	LoaderPlaceholderOrgID int64
	// LoaderPlaceholderTenantID Deprecated: 与 LoaderPlaceholderOrgID 同义，配置兼容。
	LoaderPlaceholderTenantID int64
	// RateLimit 控制 suggest HTTP 限流（按 operator；backend 可选 redis）。
	RateLimit RateLimitConfig
	// WildcardKeyCap 通配符展开的最大终端键数；0 表示使用领域默认值。
	WildcardKeyCap int
	// VisibilityCacheTTLSeconds ProfileVisibilityIDsResolver 结果缓存秒数；0=关闭。
	VisibilityCacheTTLSeconds int
}

// DefaultConfig returns the behavior-preserving defaults for suggest.
func DefaultConfig() Config {
	max := domainsuggest.DefaultLimit
	return Config{
		MaxResults:         max,
		InternalMaxResults: max * domainsuggest.DefaultInternalMult,
		KeyPadLen:          domainsuggest.DefaultKeyPadLen,
		FullSyncCron:       "@every 1h",
		DeltaSyncCron:      "",
	}
}

// WithDefaults fills unset optional values without changing explicit settings.
func (c Config) WithDefaults() Config {
	cfg := DefaultConfig()
	cfg.Enable = c.Enable
	cfg.Required = c.Required
	cfg.FullSQL = c.FullSQL
	cfg.DeltaSQL = c.DeltaSQL
	cfg.DisableMobileMask = c.DisableMobileMask
	cfg.LoaderPlaceholderOrgID = c.LoaderPlaceholderOrgID
	cfg.LoaderPlaceholderTenantID = c.LoaderPlaceholderTenantID
	if cfg.LoaderPlaceholderOrgID == 0 && cfg.LoaderPlaceholderTenantID != 0 {
		cfg.LoaderPlaceholderOrgID = cfg.LoaderPlaceholderTenantID
	}
	if c.FullSyncCron != "" {
		cfg.FullSyncCron = c.FullSyncCron
	}
	cfg.DeltaSyncCron = c.DeltaSyncCron
	if c.MaxResults > 0 {
		cfg.MaxResults = c.MaxResults
	}
	if c.InternalMaxResults > 0 {
		cfg.InternalMaxResults = c.InternalMaxResults
	} else {
		cfg.InternalMaxResults = cfg.MaxResults * domainsuggest.DefaultInternalMult
	}
	if cfg.InternalMaxResults < cfg.MaxResults {
		cfg.InternalMaxResults = cfg.MaxResults
	}
	if c.KeyPadLen > 0 {
		cfg.KeyPadLen = c.KeyPadLen
	}
	cfg.RateLimit = c.RateLimit.withDefaults()
	cfg.WildcardKeyCap = c.WildcardKeyCap
	cfg.VisibilityCacheTTLSeconds = c.VisibilityCacheTTLSeconds
	return cfg
}
