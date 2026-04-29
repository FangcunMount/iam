package process

import (
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	_ "github.com/FangcunMount/component-base/pkg/messaging/nsq" // 注册 NSQ Provider
)

// createEventBus 创建消息总线（如果配置启用了 NSQ）。
func (s *apiServer) createEventBus() (messaging.EventBus, error) {
	if s == nil || s.cfg == nil || s.cfg.NSQOptions == nil || !s.cfg.NSQOptions.Enabled {
		log.Info("📨 NSQ EventBus: disabled")
		return nil, nil
	}

	cfg := normalizeNSQConfig(s.cfg.NSQOptions.ToMessagingConfig())
	eventBus, err := messaging.NewEventBus(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSQ EventBus: %w", err)
	}

	log.Info("📨 NSQ EventBus: enabled",
		log.Strings("lookupd_addrs", cfg.NSQ.LookupdAddrs),
		log.String("nsqd_addr", cfg.NSQ.NSQdAddr),
	)

	return eventBus, nil
}

func normalizeNSQConfig(cfg *messaging.Config) *messaging.Config {
	if cfg == nil {
		cfg = &messaging.Config{}
	}
	if cfg.NSQ.MsgTimeout == 0 {
		cfg.NSQ.MsgTimeout = 60 * time.Second
	}
	if cfg.NSQ.RequeueDelay == 0 {
		cfg.NSQ.RequeueDelay = 5 * time.Second
	}
	if len(cfg.NSQ.LookupdAddrs) == 0 {
		cfg.NSQ.LookupdAddrs = []string{"127.0.0.1:4161"}
	}
	if cfg.NSQ.NSQdAddr == "" {
		cfg.NSQ.NSQdAddr = "127.0.0.1:4150"
	}
	if cfg.NSQ.MaxAttempts == 0 {
		cfg.NSQ.MaxAttempts = 5
	}
	if cfg.NSQ.MaxInFlight == 0 {
		cfg.NSQ.MaxInFlight = 200
	}
	return cfg
}
