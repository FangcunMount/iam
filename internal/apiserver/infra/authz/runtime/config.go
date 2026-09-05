package runtime

import (
	"fmt"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
)

type Config struct {
	CheckInterval  time.Duration `mapstructure:"check-interval" json:"check_interval"`
	SyncTimeout    time.Duration `mapstructure:"sync-timeout" json:"sync_timeout"`
	MaxUnconfirmed time.Duration `mapstructure:"max-unconfirmed" json:"max_unconfirmed"`
}

func DefaultConfig() Config { return Config{10 * time.Second, 10 * time.Second, 60 * time.Second} }
func (c Config) Validate() error {
	if c.CheckInterval <= 0 || c.SyncTimeout <= 0 || c.MaxUnconfirmed <= 0 || c.CheckInterval >= c.MaxUnconfirmed || c.SyncTimeout >= c.MaxUnconfirmed {
		return fmt.Errorf("authz intervals must be positive and check interval/sync timeout must be less than max unconfirmed age")
	}
	return nil
}

type Option func(*Runtime)

func WithConfig(c Config) Option { return func(r *Runtime) { r.config = c } }

// WithClock allows deterministic freshness tests without sleeping.
func WithClock(now func() time.Time) Option { return func(r *Runtime) { r.now = now } }

func WithAttributeProviders(p objectattributeadmission.Coverage) Option {
	return func(r *Runtime) { r.providers = p }
}
