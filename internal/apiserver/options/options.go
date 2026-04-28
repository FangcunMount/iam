package options

import (
	"encoding/json"

	"github.com/FangcunMount/component-base/pkg/log"
	genericoptions "github.com/FangcunMount/iam/internal/pkg/options"
	cliflag "github.com/FangcunMount/iam/pkg/flag"
)

// Options 包含所有配置项
type Options struct {
	Log                     *log.Options                           `json:"log"    mapstructure:"log"`
	GenericServerRunOptions *genericoptions.ServerRunOptions       `json:"server" mapstructure:"server"`
	GRPCOptions             *genericoptions.GRPCOptions            `json:"grpc"     mapstructure:"grpc"`
	InsecureServing         *genericoptions.InsecureServingOptions `json:"insecure" mapstructure:"insecure"`
	SecureServing           *genericoptions.SecureServingOptions   `json:"secure" mapstructure:"secure"`
	MySQLOptions            *genericoptions.MySQLOptions           `json:"mysql"    mapstructure:"mysql"`
	RedisOptions            *genericoptions.RedisOptions           `json:"redis"    mapstructure:"redis"`
	NSQOptions              *genericoptions.NSQOptions             `json:"nsq"      mapstructure:"nsq"`
	MigrationOptions        *genericoptions.MigrationOptions       `json:"migration" mapstructure:"migration"`
}

// NewOptions 创建一个 Options 对象，包含默认参数
func NewOptions() *Options {
	return &Options{
		Log:                     log.NewOptions(),
		GenericServerRunOptions: genericoptions.NewServerRunOptions(),
		GRPCOptions:             genericoptions.NewGRPCOptions(),
		InsecureServing:         genericoptions.NewInsecureServingOptions(),
		SecureServing:           genericoptions.NewSecureServingOptions(),
		MySQLOptions:            genericoptions.NewMySQLOptions(),
		RedisOptions:            genericoptions.NewRedisOptions(),
		NSQOptions:              genericoptions.NewNSQOptions(),
		MigrationOptions:        genericoptions.NewMigrationOptions(),
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
	return o.SecureServing.Complete()
}

// String 返回配置的字符串表示
func (o *Options) String() string {
	data, _ := json.Marshal(o)
	return string(data)
}
