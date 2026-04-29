package suggest

// Config 控制 suggest 模块行为
type Config struct {
	Enable        bool
	DataDir       string
	FullSyncCron  string
	DeltaSyncCron string
	MaxResults    int
	KeyPadLen     int
	FullSQL       string
	DeltaSQL      string
	Snapshot      bool
}

// DefaultConfig returns the behavior-preserving defaults for suggest.
func DefaultConfig() Config {
	return Config{
		MaxResults:    20,
		KeyPadLen:     25,
		FullSyncCron:  "@every 1h",
		DeltaSyncCron: "",
	}
}

// WithDefaults fills unset optional values without changing explicit settings.
func (c Config) WithDefaults() Config {
	cfg := DefaultConfig()
	cfg.Enable = c.Enable
	cfg.DataDir = c.DataDir
	cfg.FullSQL = c.FullSQL
	cfg.DeltaSQL = c.DeltaSQL
	cfg.Snapshot = c.Snapshot
	if c.FullSyncCron != "" {
		cfg.FullSyncCron = c.FullSyncCron
	}
	cfg.DeltaSyncCron = c.DeltaSyncCron
	if c.MaxResults > 0 {
		cfg.MaxResults = c.MaxResults
	}
	if c.KeyPadLen > 0 {
		cfg.KeyPadLen = c.KeyPadLen
	}
	return cfg
}

// ToUpdaterConfig 转换为 Updater 配置
func (c Config) ToUpdaterConfig() UpdaterConfig {
	return UpdaterConfig{
		FullCron:  c.FullSyncCron,
		DeltaCron: c.DeltaSyncCron,
		DataDir:   c.DataDir,
		Snapshot:  c.Snapshot,
	}
}
