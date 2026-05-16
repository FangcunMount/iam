package suggest

import domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"

// Config 控制 suggest 模块行为
type Config struct {
	Enable             bool
	Required           bool
	DataDir            string
	FullSyncCron       string
	DeltaSyncCron      string
	MaxResults         int
	InternalMaxResults int
	KeyPadLen          int
	FullSQL            string
	DeltaSQL           string
	Snapshot           bool
	// DisableMobileMask 为 true 时返回明文手机号（仅特殊排障；默认应关闭）。
	DisableMobileMask bool
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
	cfg.DataDir = c.DataDir
	cfg.FullSQL = c.FullSQL
	cfg.DeltaSQL = c.DeltaSQL
	cfg.Snapshot = c.Snapshot
	cfg.DisableMobileMask = c.DisableMobileMask
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
	return cfg
}
