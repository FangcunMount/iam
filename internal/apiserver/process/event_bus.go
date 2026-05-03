package process

import (
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	_ "github.com/FangcunMount/component-base/pkg/messaging/nsq" // 注册 NSQ Provider
)

// createEventBus 创建事件总线（如果配置启用了 NSQ）
func (s *apiServer) createEventBus() (messaging.EventBus, error) {
	if s == nil || s.cfg == nil || s.cfg.NSQOptions == nil || !s.cfg.NSQOptions.Enabled {
		log.Info("📨 NSQ EventBus: disabled")
		return nil, nil
	}

	// 规范化 NSQ 配置
	cfg := normalizeNSQConfig(s.cfg.NSQOptions.ToMessagingConfig())
	// 创建事件总线
	eventBus, err := messaging.NewEventBus(cfg)
	if err != nil {
		// 如果创建事件总线失败，则返回错误
		return nil, fmt.Errorf("failed to create NSQ EventBus: %w", err)
	}

	log.Info("📨 NSQ EventBus: enabled",
		log.Strings("lookupd_addrs", cfg.NSQ.LookupdAddrs),
		log.String("nsqd_addr", cfg.NSQ.NSQdAddr),
	)

	// 返回事件总线
	return eventBus, nil
}

// normalizeNSQConfig 规范化 NSQ 配置
func normalizeNSQConfig(cfg *messaging.Config) *messaging.Config {
	// 如果配置为空，则创建默认配置
	if cfg == nil {
		cfg = &messaging.Config{}
	}
	// 设置默认消息超时时间
	if cfg.NSQ.MsgTimeout == 0 {
		cfg.NSQ.MsgTimeout = 60 * time.Second
	}
	// 设置默认重试延迟时间
	if cfg.NSQ.RequeueDelay == 0 {
		cfg.NSQ.RequeueDelay = 5 * time.Second
	}
	// 设置默认查找地址
	if len(cfg.NSQ.LookupdAddrs) == 0 {
		cfg.NSQ.LookupdAddrs = []string{"127.0.0.1:4161"}
	}
	// 设置默认 NSQ 地址
	if cfg.NSQ.NSQdAddr == "" {
		cfg.NSQ.NSQdAddr = "127.0.0.1:4150"
	}
	// 设置默认最大重试次数
	if cfg.NSQ.MaxAttempts == 0 {
		cfg.NSQ.MaxAttempts = 5
	}
	// 设置默认最大并发处理数
	if cfg.NSQ.MaxInFlight == 0 {
		cfg.NSQ.MaxInFlight = 200
	}
	// 返回规范化后的配置
	return cfg
}
