package options

import (
	"github.com/spf13/pflag"
)

// NSQOptions NSQ 消息队列配置选项
type NSQOptions struct {
	// Enabled 是否启用 NSQ
	Enabled bool `json:"enabled" mapstructure:"enabled"`
	// ConsumerSDKEnabled switches IAM policy notifications to reliable-messaging.
	ConsumerSDKEnabled bool `json:"consumer-sdk-enabled" mapstructure:"consumer-sdk-enabled"`
	// NSQdHTTPAddrs lists the nsqd HTTP endpoints to prepare durable failure channels.
	NSQdHTTPAddrs []string `json:"nsqd-http-addrs" mapstructure:"nsqd-http-addrs"`

	// LookupdAddrs NSQLookupd 地址列表
	LookupdAddrs []string `json:"lookupd-addrs" mapstructure:"lookupd-addrs"`

	// NSQdAddr NSQd 地址（用于发布）
	NSQdAddr string `json:"nsqd-addr" mapstructure:"nsqd-addr"`

	// MaxAttempts 最大消息重试次数
	MaxAttempts uint16 `json:"max-attempts" mapstructure:"max-attempts"`

	// MaxInFlight 最大消息处理并发数
	MaxInFlight int `json:"max-in-flight" mapstructure:"max-in-flight"`

	// MsgTimeout 消息超时时间（秒）
	MsgTimeout int `json:"msg-timeout" mapstructure:"msg-timeout"`

	// RequeueDelay 重新入队延迟（秒）
	RequeueDelay int `json:"requeue-delay" mapstructure:"requeue-delay"`
}

// NewNSQOptions 创建默认的 NSQ 配置
func NewNSQOptions() *NSQOptions {
	return &NSQOptions{
		Enabled:      false, // 默认不启用，需要显式开启
		LookupdAddrs: []string{"127.0.0.1:4161"},
		NSQdAddr:     "127.0.0.1:4150",
		MaxAttempts:  5,
		MaxInFlight:  200,
		MsgTimeout:   60, // 60 秒
		RequeueDelay: 5,  // 5 秒
	}
}

// Validate 验证 NSQ 配置
func (o *NSQOptions) Validate() []error {
	var errs []error

	if o.Enabled {
		if len(o.LookupdAddrs) == 0 && o.NSQdAddr == "" {
			// 至少需要一个地址
		}
	}

	return errs
}

// AddFlags 添加 NSQ 相关的命令行参数
func (o *NSQOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.Enabled, "nsq.enabled", o.Enabled,
		"Enable NSQ message queue for event-driven features.")
	fs.BoolVar(&o.ConsumerSDKEnabled, "nsq.consumer-sdk-enabled", o.ConsumerSDKEnabled,
		"Use reliable-messaging for IAM policy notification consumption.")
	fs.StringSliceVar(&o.NSQdHTTPAddrs, "nsq.nsqd-http-addrs", o.NSQdHTTPAddrs,
		"Explicit nsqd HTTP endpoints for durable failure channel preparation.")

	fs.StringSliceVar(&o.LookupdAddrs, "nsq.lookupd-addrs", o.LookupdAddrs,
		"NSQLookupd addresses for consumer discovery.")

	fs.StringVar(&o.NSQdAddr, "nsq.nsqd-addr", o.NSQdAddr,
		"NSQd address for publishing messages.")

	fs.Uint16Var(&o.MaxAttempts, "nsq.max-attempts", o.MaxAttempts,
		"Maximum message retry attempts.")

	fs.IntVar(&o.MaxInFlight, "nsq.max-in-flight", o.MaxInFlight,
		"Maximum number of messages to process concurrently.")

	fs.IntVar(&o.MsgTimeout, "nsq.msg-timeout", o.MsgTimeout,
		"Message processing timeout in seconds.")

	fs.IntVar(&o.RequeueDelay, "nsq.requeue-delay", o.RequeueDelay,
		"Delay before requeuing a failed message in seconds.")
}
