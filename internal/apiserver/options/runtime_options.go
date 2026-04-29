package options

import "time"

// AppOptions captures runtime application metadata used by bootstrap code.
type AppOptions struct {
	Name    string `json:"name" mapstructure:"name"`
	Version string `json:"version" mapstructure:"version"`
	Mode    string `json:"mode" mapstructure:"mode"`
}

func NewAppOptions() *AppOptions {
	return &AppOptions{
		Name:    "iam",
		Version: "1.0.0",
		Mode:    "development",
	}
}

// AuthOptions configures JWT token issuing and verification.
type AuthOptions struct {
	JWTIssuer           string        `json:"jwt_issuer" mapstructure:"jwt_issuer"`
	AccessTokenAudience []string      `json:"access_token_audience" mapstructure:"access_token_audience"`
	AccessTokenTTL      time.Duration `json:"access_token_ttl" mapstructure:"access_token_ttl"`
	RefreshTokenTTL     time.Duration `json:"refresh_token_ttl" mapstructure:"refresh_token_ttl"`
}

func NewAuthOptions() *AuthOptions {
	return &AuthOptions{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

// JWKSOptions configures key storage and startup key initialization.
type JWKSOptions struct {
	KeysDir  string `json:"keys_dir" mapstructure:"keys_dir"`
	AutoInit bool   `json:"auto_init" mapstructure:"auto_init"`
}

func NewJWKSOptions() *JWKSOptions {
	return &JWKSOptions{}
}

// IDPOptions configures identity provider bootstrap secrets.
type IDPOptions struct {
	EncryptionKey string `json:"encryption-key" mapstructure:"encryption-key"`
}

func NewIDPOptions() *IDPOptions {
	return &IDPOptions{}
}

// SMSOptions configures login OTP delivery.
type SMSOptions struct {
	Provider             string        `json:"provider" mapstructure:"provider"`
	LoginOTPTTL          time.Duration `json:"login_otp_ttl" mapstructure:"login_otp_ttl"`
	LoginOTPSendCooldown time.Duration `json:"login_otp_send_cooldown" mapstructure:"login_otp_send_cooldown"`
	LoginOTPCodeLength   int           `json:"login_otp_code_length" mapstructure:"login_otp_code_length"`
	MQ                   SMSMQOptions  `json:"mq" mapstructure:"mq"`
}

type SMSMQOptions struct {
	Topic string `json:"topic" mapstructure:"topic"`
}

func NewSMSOptions() *SMSOptions {
	return &SMSOptions{
		Provider:             "log",
		LoginOTPTTL:          5 * time.Minute,
		LoginOTPSendCooldown: 60 * time.Second,
		LoginOTPCodeLength:   6,
		MQ: SMSMQOptions{
			Topic: "iam.notify.sms",
		},
	}
}

// SuggestOptions configures the suggest module and its refresh loop.
type SuggestOptions struct {
	Enable        bool   `json:"enable" mapstructure:"enable"`
	DataDir       string `json:"data_dir" mapstructure:"data_dir"`
	FullSyncCron  string `json:"full_sync_cron" mapstructure:"full_sync_cron"`
	DeltaSyncCron string `json:"delta_sync_cron" mapstructure:"delta_sync_cron"`
	MaxResults    int    `json:"max_results" mapstructure:"max_results"`
	KeyPadLen     int    `json:"key_pad_len" mapstructure:"key_pad_len"`
	FullSQL       string `json:"full_sql" mapstructure:"full_sql"`
	DeltaSQL      string `json:"delta_sql" mapstructure:"delta_sql"`
	Snapshot      *bool  `json:"snapshot" mapstructure:"snapshot"`
}

func NewSuggestOptions() *SuggestOptions {
	return &SuggestOptions{
		FullSyncCron: "@every 1h",
		MaxResults:   20,
		KeyPadLen:    25,
	}
}

// DebugOptions configures debug-only HTTP surfaces.
type DebugOptions struct {
	CacheGovernance CacheGovernanceDebugOptions `json:"cache_governance" mapstructure:"cache_governance"`
}

type CacheGovernanceDebugOptions struct {
	Enabled      *bool `json:"enabled" mapstructure:"enabled"`
	RequireAdmin *bool `json:"require_admin" mapstructure:"require_admin"`
}

func NewDebugOptions() *DebugOptions {
	return &DebugOptions{}
}

// SeedMockAuthOptions controls the internal seed/mock consumer auth route.
type SeedMockAuthOptions struct {
	Enabled      bool   `json:"enabled" mapstructure:"enabled"`
	SharedSecret string `json:"shared_secret" mapstructure:"shared_secret"`
}

func NewSeedMockAuthOptions() *SeedMockAuthOptions {
	return &SeedMockAuthOptions{}
}

// EventOptions configures event catalog loading and durable outbox relay timing.
type EventOptions struct {
	CatalogPath           string        `json:"catalog_path" mapstructure:"catalog_path"`
	OutboxRelayInterval   time.Duration `json:"outbox_relay_interval" mapstructure:"outbox_relay_interval"`
	OutboxRelayBatchSize  int           `json:"outbox_relay_batch_size" mapstructure:"outbox_relay_batch_size"`
	OutboxRelayRetryDelay time.Duration `json:"outbox_relay_retry_delay" mapstructure:"outbox_relay_retry_delay"`
}

func NewEventOptions() *EventOptions {
	return &EventOptions{
		CatalogPath:         "configs/events.yaml",
		OutboxRelayInterval: 2 * time.Second,
	}
}
