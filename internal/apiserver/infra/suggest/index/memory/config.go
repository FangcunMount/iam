package memory

const (
	DefaultKeyPadLen      = 25
	DefaultWildcardKeyCap = 100
)

// Config 控制内存索引 TST/Hash 行为。
type Config struct {
	KeyPadLen      int
	WildcardKeyCap int
}

// WithDefaults 填充默认值。
func (c Config) WithDefaults() Config {
	out := c
	if out.KeyPadLen <= 0 {
		out.KeyPadLen = DefaultKeyPadLen
	}
	if out.WildcardKeyCap <= 0 {
		out.WildcardKeyCap = DefaultWildcardKeyCap
	}
	return out
}
