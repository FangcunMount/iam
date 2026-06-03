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
	SessionMaxTTL       time.Duration `json:"session_max_ttl" mapstructure:"session_max_ttl"`
}

func NewAuthOptions() *AuthOptions {
	return &AuthOptions{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		SessionMaxTTL:   24 * time.Hour,
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
	EncryptionKey string            `json:"encryption-key" mapstructure:"encryption-key"`
	WeCom         WeComOptions      `json:"wecom" mapstructure:"wecom"`
	WechatOpen    WechatOpenOptions `json:"wechat_open" mapstructure:"wechat_open"`
}

func NewIDPOptions() *IDPOptions {
	return &IDPOptions{}
}

// WeComOptions configures server-side Enterprise WeChat login material that
// should not be supplied by public login requests.
type WeComOptions struct {
	AgentID string `json:"agent_id" mapstructure:"agent_id"`
}

// WechatOpenOptions configures server-side WeChat Open Platform (website QR
// login) material. AppID and redirect URIs must be configured server-side and
// never be supplied by public login or linking requests; app_secret is resolved
// via the WeChat app store keyed by AppID.
//
// 登录与绑定使用不同的回调地址：登录回调是公开页面，绑定回调是登录态页面，
// 两者分开便于各自锁死白名单与页面路由。
type WechatOpenOptions struct {
	AppID            string `json:"app_id" mapstructure:"app_id"`
	LoginRedirectURI string `json:"login_redirect_uri" mapstructure:"login_redirect_uri"`
	LinkRedirectURI  string `json:"link_redirect_uri" mapstructure:"link_redirect_uri"`
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
	Enable             bool   `json:"enable" mapstructure:"enable"`
	Required           bool   `json:"required" mapstructure:"required"`
	DataDir            string `json:"data_dir" mapstructure:"data_dir"`
	FullSyncCron       string `json:"full_sync_cron" mapstructure:"full_sync_cron"`
	DeltaSyncCron      string `json:"delta_sync_cron" mapstructure:"delta_sync_cron"`
	MaxResults         int    `json:"max_results" mapstructure:"max_results"`
	InternalMaxResults int    `json:"internal_max_results" mapstructure:"internal_max_results"`
	KeyPadLen          int    `json:"key_pad_len" mapstructure:"key_pad_len"`
	FullSQL            string `json:"full_sql" mapstructure:"full_sql"`
	DeltaSQL           string `json:"delta_sql" mapstructure:"delta_sql"`
	Snapshot           *bool  `json:"snapshot" mapstructure:"snapshot"`
	DisableMobileMask  bool   `json:"disable_mobile_mask" mapstructure:"disable_mobile_mask"`
	// LoaderPlaceholderOrgID 内建 Loader 注入的 org_id；0 表示索引不虚构组织维度。
	LoaderPlaceholderOrgID int64 `json:"loader_placeholder_org_id" mapstructure:"loader_placeholder_org_id"`
	// LoaderPlaceholderTenantID Deprecated: 与 loader_placeholder_org_id 同义。
	LoaderPlaceholderTenantID int64 `json:"loader_placeholder_tenant_id" mapstructure:"loader_placeholder_tenant_id"`
	// WildcardKeyCap 通配符展开的最大终端键数；0 使用领域默认。
	WildcardKeyCap int `json:"trie_wildcard_key_cap" mapstructure:"trie_wildcard_key_cap"`
	// RateLimit REST 按操作员限流；全零表示关闭。
	RateLimit struct {
		PerOperatorQPS                float64 `json:"per_operator_qps" mapstructure:"per_operator_qps"`
		PerOperatorBurst              int     `json:"per_operator_burst" mapstructure:"per_operator_burst"`
		MobileKeywordPerOperatorQPS   float64 `json:"mobile_keyword_per_operator_qps" mapstructure:"mobile_keyword_per_operator_qps"`
		MobileKeywordPerOperatorBurst int     `json:"mobile_keyword_per_operator_burst" mapstructure:"mobile_keyword_per_operator_burst"`
		Backend                       string  `json:"backend" mapstructure:"backend"`
		OperatorMapMaxEntries         int     `json:"operator_map_max_entries" mapstructure:"operator_map_max_entries"`
	} `json:"rate_limit" mapstructure:"rate_limit"`
	// VisibilityCacheTTLSeconds 可见 ProfileID 查询缓存秒数；0=关闭。
	VisibilityCacheTTLSeconds int `json:"visibility_cache_ttl_seconds" mapstructure:"visibility_cache_ttl_seconds"`
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
