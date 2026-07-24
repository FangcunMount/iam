package options

import "time"

// RemovedAppOptions captures removed app.* keys solely so startup can reject
// stale configuration instead of silently ignoring it.
type RemovedAppOptions struct {
	Name    *string `json:"-" mapstructure:"name"`
	Version *string `json:"-" mapstructure:"version"`
	Mode    *string `json:"-" mapstructure:"mode"`
}

// AuthOptions configures JWT token issuing and verification.
type AuthOptions struct {
	JWTIssuer           string                 `json:"jwt_issuer" mapstructure:"jwt_issuer"`
	AccessTokenAudience []string               `json:"access_token_audience" mapstructure:"access_token_audience"`
	AccessTokenTTL      time.Duration          `json:"access_token_ttl" mapstructure:"access_token_ttl"`
	RefreshTokenTTL     time.Duration          `json:"refresh_token_ttl" mapstructure:"refresh_token_ttl"`
	SessionMaxTTL       time.Duration          `json:"session_max_ttl" mapstructure:"session_max_ttl"`
	PasswordLockout     PasswordLockoutOptions `json:"password_lockout" mapstructure:"password_lockout"`
}

// PasswordLockoutOptions configures consecutive password failure lockout.
type PasswordLockoutOptions struct {
	Enabled      bool          `json:"enabled" mapstructure:"enabled"`
	Threshold    int           `json:"threshold" mapstructure:"threshold"`
	LockDuration time.Duration `json:"lock_duration" mapstructure:"lock_duration"`
}

func NewAuthOptions() *AuthOptions {
	return &AuthOptions{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		SessionMaxTTL:   24 * time.Hour,
		PasswordLockout: PasswordLockoutOptions{
			Enabled:      false,
			Threshold:    5,
			LockDuration: 15 * time.Minute,
		},
	}
}

// JWKSOptions configures key storage and startup key initialization.
type JWKSOptions struct {
	KeysDir  string              `json:"keys_dir" mapstructure:"keys_dir"`
	AutoInit bool                `json:"auto_init" mapstructure:"auto_init"`
	Rotation JWKSRotationOptions `json:"rotation" mapstructure:"rotation"`
}

// JWKSRotationOptions configures automatic signing-key rotation.
type JWKSRotationOptions struct {
	AutomaticEnabled  bool          `json:"automatic_enabled" mapstructure:"automatic_enabled"`
	CheckCron         string        `json:"check_cron" mapstructure:"check_cron"`
	RotationInterval  time.Duration `json:"rotation_interval" mapstructure:"rotation_interval"`
	GracePeriod       time.Duration `json:"grace_period" mapstructure:"grace_period"`
	MaxPublishableKey int           `json:"max_publishable_keys" mapstructure:"max_publishable_keys"`
}

func NewJWKSOptions() *JWKSOptions {
	return &JWKSOptions{
		Rotation: JWKSRotationOptions{
			CheckCron:         "@every 1h",
			RotationInterval:  30 * 24 * time.Hour,
			GracePeriod:       7 * 24 * time.Hour,
			MaxPublishableKey: 3,
		},
	}
}

// IDPOptions configures identity provider bootstrap secrets.
type IDPOptions struct {
	EncryptionKey string            `json:"-" mapstructure:"encryption-key"`
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
	// 限量：单号码+场景滑动窗口发送上限。<0 关闭，0 取默认（小时 5、每天 10）。
	LoginOTPHourlyLimit int              `json:"login_otp_hourly_limit" mapstructure:"login_otp_hourly_limit"`
	LoginOTPDailyLimit  int              `json:"login_otp_daily_limit" mapstructure:"login_otp_daily_limit"`
	MQ                  SMSMQOptions     `json:"mq" mapstructure:"mq"`
	Aliyun              SMSAliyunOptions `json:"aliyun" mapstructure:"aliyun"`
}

type SMSMQOptions struct {
	Topic string `json:"topic" mapstructure:"topic"`
}

// SMSAliyunOptions configures Aliyun Dypns (号码认证) SendSmsVerifyCode delivery.
// AccessKeyID/AccessKeySecret 应通过环境变量注入，禁止明文写入配置文件。
type SMSAliyunOptions struct {
	// AccessKeyID/AccessKeySecret 留空时走阿里云默认凭据链（环境变量/RAM 角色等）。
	AccessKeyID     string `json:"-" mapstructure:"access_key_id"`
	AccessKeySecret string `json:"-" mapstructure:"access_key_secret"`
	SignName        string `json:"sign_name" mapstructure:"sign_name"`
	TemplateCode    string `json:"template_code" mapstructure:"template_code"`
	Endpoint        string `json:"endpoint" mapstructure:"endpoint"`
	CodeParamName   string `json:"code_param_name" mapstructure:"code_param_name"`
	MinParamName    string `json:"min_param_name" mapstructure:"min_param_name"`
	TimeoutMillis   int    `json:"timeout_millis" mapstructure:"timeout_millis"`
}

func NewSMSOptions() *SMSOptions {
	return &SMSOptions{
		Provider:             "log",
		LoginOTPTTL:          5 * time.Minute,
		LoginOTPSendCooldown: 60 * time.Second,
		LoginOTPCodeLength:   6,
		LoginOTPHourlyLimit:  5,
		LoginOTPDailyLimit:   10,
		MQ: SMSMQOptions{
			Topic: "iam.notify.sms",
		},
		Aliyun: SMSAliyunOptions{
			Endpoint:      "dypnsapi.aliyuncs.com",
			CodeParamName: "code",
			MinParamName:  "min",
		},
	}
}

// SuggestOptions configures the suggest module and its refresh loop.
type SuggestOptions struct {
	Enable             bool   `json:"enable" mapstructure:"enable"`
	Required           bool   `json:"required" mapstructure:"required"`
	FullSyncCron       string `json:"full_sync_cron" mapstructure:"full_sync_cron"`
	DeltaSyncCron      string `json:"delta_sync_cron" mapstructure:"delta_sync_cron"`
	MaxResults         int    `json:"max_results" mapstructure:"max_results"`
	InternalMaxResults int    `json:"internal_max_results" mapstructure:"internal_max_results"`
	KeyPadLen          int    `json:"key_pad_len" mapstructure:"key_pad_len"`
	FullSQL            string `json:"full_sql" mapstructure:"full_sql"`
	DeltaSQL           string `json:"delta_sql" mapstructure:"delta_sql"`
	DisableMobileMask  bool   `json:"disable_mobile_mask" mapstructure:"disable_mobile_mask"`
	// RemovedDataDir and RemovedSnapshot are decode-only tombstones. They make
	// retired configuration fail closed instead of being silently ignored.
	RemovedDataDir  *string `json:"-" mapstructure:"data_dir"`
	RemovedSnapshot *bool   `json:"-" mapstructure:"snapshot"`
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
	SharedSecret string `json:"-" mapstructure:"shared_secret"`
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
