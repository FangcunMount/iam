package options

import (
	"encoding/json"

	"github.com/FangcunMount/component-base/pkg/log"
	genericoptions "github.com/FangcunMount/iam/v2/internal/pkg/options"
	cliflag "github.com/FangcunMount/iam/v2/pkg/flag"
)

// Options 包含所有配置项
type Options struct {
	Log                     *log.Options                           `json:"log"    mapstructure:"log"`
	App                     *AppOptions                            `json:"app" mapstructure:"app"`
	GenericServerRunOptions *genericoptions.ServerRunOptions       `json:"server" mapstructure:"server"`
	GRPCOptions             *genericoptions.GRPCOptions            `json:"grpc"     mapstructure:"grpc"`
	InsecureServing         *genericoptions.InsecureServingOptions `json:"insecure" mapstructure:"insecure"`
	SecureServing           *genericoptions.SecureServingOptions   `json:"secure" mapstructure:"secure"`
	MySQLOptions            *genericoptions.MySQLOptions           `json:"mysql"    mapstructure:"mysql"`
	RedisOptions            *genericoptions.RedisOptions           `json:"redis"    mapstructure:"redis"`
	NSQOptions              *genericoptions.NSQOptions             `json:"nsq"      mapstructure:"nsq"`
	MigrationOptions        *genericoptions.MigrationOptions       `json:"migration" mapstructure:"migration"`
	Auth                    *AuthOptions                           `json:"auth" mapstructure:"auth"`
	JWKS                    *JWKSOptions                           `json:"jwks" mapstructure:"jwks"`
	IDP                     *IDPOptions                            `json:"idp" mapstructure:"idp"`
	SMS                     *SMSOptions                            `json:"sms" mapstructure:"sms"`
	Suggest                 *SuggestOptions                        `json:"suggest" mapstructure:"suggest"`
	Debug                   *DebugOptions                          `json:"debug" mapstructure:"debug"`
	SeedMockAuth            *SeedMockAuthOptions                   `json:"seed_mock_auth" mapstructure:"seed_mock_auth"`
	Events                  *EventOptions                          `json:"events" mapstructure:"events"`
}

// NewOptions 创建一个 Options 对象，包含默认参数
func NewOptions() *Options {
	return &Options{
		Log:                     log.NewOptions(),
		App:                     NewAppOptions(),
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		GRPCOptions:             genericoptions.NewGRPCOptions(),
		InsecureServing:         genericoptions.NewInsecureServingOptions(),
		SecureServing:           genericoptions.NewSecureServingOptions(),
		MySQLOptions:            genericoptions.NewMySQLOptions(),
		RedisOptions:            genericoptions.NewRedisOptions(),
		NSQOptions:              genericoptions.NewNSQOptions(),
		MigrationOptions:        genericoptions.NewMigrationOptions(),
		Auth:                    NewAuthOptions(),
		JWKS:                    NewJWKSOptions(),
		IDP:                     NewIDPOptions(),
		SMS:                     NewSMSOptions(),
		Suggest:                 NewSuggestOptions(),
		Debug:                   NewDebugOptions(),
		SeedMockAuth:            NewSeedMockAuthOptions(),
		Events:                  NewEventOptions(),
	}
}

// Flags 返回一个 NamedFlagSets 对象，包含所有命令行参数
func (o *Options) Flags() (fss cliflag.NamedFlagSets) {
	o.Log.AddFlags(fss.FlagSet("log"))
	o.GenericServerRunOptions.AddFlags(fss.FlagSet("server"))
	o.GRPCOptions.AddFlags(fss.FlagSet("grpc"))
	o.InsecureServing.AddFlags(fss.FlagSet("insecure"))
	o.SecureServing.AddFlags(fss.FlagSet("secure"))
	o.MySQLOptions.AddFlags(fss.FlagSet("mysql"))
	o.RedisOptions.AddFlags(fss.FlagSet("redis"))
	o.NSQOptions.AddFlags(fss.FlagSet("nsq"))
	o.MigrationOptions.AddFlags(fss.FlagSet("migration"))

	return fss
}

// Complete 完成配置选项
func (o *Options) Complete() error {
	o.ApplyDefaults()
	return o.SecureServing.Complete()
}

// String 返回配置的字符串表示
func (o *Options) String() string {
	data, _ := json.Marshal(o)
	return string(data)
}

// ApplyDefaults fills nil option groups with their default values.
func (o *Options) ApplyDefaults() {
	if o.Log == nil {
		o.Log = log.NewOptions()
	}
	if o.App == nil {
		o.App = NewAppOptions()
	}
	if o.GenericServerRunOptions == nil {
		o.GenericServerRunOptions = genericoptions.NewServerRunOptions()
	}
	if o.GRPCOptions == nil {
		o.GRPCOptions = genericoptions.NewGRPCOptions()
	}
	if o.InsecureServing == nil {
		o.InsecureServing = genericoptions.NewInsecureServingOptions()
	}
	if o.SecureServing == nil {
		o.SecureServing = genericoptions.NewSecureServingOptions()
	}
	if o.MySQLOptions == nil {
		o.MySQLOptions = genericoptions.NewMySQLOptions()
	}
	if o.RedisOptions == nil {
		o.RedisOptions = genericoptions.NewRedisOptions()
	}
	if o.NSQOptions == nil {
		o.NSQOptions = genericoptions.NewNSQOptions()
	}
	if o.MigrationOptions == nil {
		o.MigrationOptions = genericoptions.NewMigrationOptions()
	}
	if o.Auth == nil {
		o.Auth = NewAuthOptions()
	}
	if o.JWKS == nil {
		o.JWKS = NewJWKSOptions()
	}
	if o.IDP == nil {
		o.IDP = NewIDPOptions()
	}
	if o.SMS == nil {
		o.SMS = NewSMSOptions()
	}
	if o.Suggest == nil {
		o.Suggest = NewSuggestOptions()
	}
	if o.Debug == nil {
		o.Debug = NewDebugOptions()
	}
	if o.SeedMockAuth == nil {
		o.SeedMockAuth = NewSeedMockAuthOptions()
	}
	if o.Events == nil {
		o.Events = NewEventOptions()
	}
}
