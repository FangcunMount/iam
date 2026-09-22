package options

import (
	"errors"
	"time"
)

// ReliableMessagingOptions only selects the IAM policy Outbox adapter. It does
// not change broker or authorize overlap with legacy writers on other processes.
type ReliableMessagingOptions struct {
	Enabled         bool          `json:"enabled" mapstructure:"enabled"`
	Concurrency     int           `json:"concurrency" mapstructure:"concurrency"`
	Lease           time.Duration `json:"lease" mapstructure:"lease"`
	PublishTimeout  time.Duration `json:"publish_timeout" mapstructure:"publish_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout" mapstructure:"write_timeout"`
	RestartDelay    time.Duration `json:"restart_delay" mapstructure:"restart_delay"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" mapstructure:"shutdown_timeout"`
	LegacyStale     time.Duration `json:"legacy_stale" mapstructure:"legacy_stale"`
}

func DefaultReliableMessagingOptions() ReliableMessagingOptions {
	return ReliableMessagingOptions{Concurrency: 1, Lease: 30 * time.Second,
		PublishTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		RestartDelay: 2 * time.Second, ShutdownTimeout: 20 * time.Second, LegacyStale: time.Minute}
}

func (o ReliableMessagingOptions) Validate() error {
	if !o.Enabled {
		return nil
	}
	if o.Concurrency < 1 || o.Concurrency > 1000 || o.Lease <= 0 || o.Lease > 24*time.Hour ||
		o.PublishTimeout < 2*time.Second || o.PublishTimeout >= o.Lease || o.WriteTimeout <= 0 || o.WriteTimeout >= o.Lease-o.PublishTimeout ||
		o.RestartDelay <= 0 || o.RestartDelay > time.Minute || o.LegacyStale <= 0 || o.LegacyStale > 24*time.Hour ||
		o.ShutdownTimeout <= o.PublishTimeout+o.WriteTimeout || o.ShutdownTimeout > 5*time.Minute {
		return errors.New("invalid events.reliable_messaging bounds: positive durations, publish >= 2s, concurrency 1..1000, lease and shutdown must exceed publish plus write")
	}
	return nil
}
