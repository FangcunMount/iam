package process

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	nsqmessaging "github.com/FangcunMount/component-base/pkg/messaging/nsq" // 注册 NSQ Provider
	"github.com/FangcunMount/iam/v2/pkg/eventcatalog"
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
	if err := s.ensureDurableTopics(cfg.NSQ.NSQdAddr); err != nil {
		_ = eventBus.Close()
		return nil, err
	}

	log.Info("📨 NSQ EventBus: enabled",
		log.Strings("lookupd_addrs", cfg.NSQ.LookupdAddrs),
		log.String("nsqd_addr", cfg.NSQ.NSQdAddr),
	)

	// 返回事件总线
	return eventBus, nil
}

func (s *apiServer) ensureDurableTopics(nsqdAddr string) error {
	if s == nil || s.cfg == nil || s.cfg.Options == nil || s.cfg.Options.Events == nil {
		return nil
	}
	topics, err := durableTopicNamesFromCatalog(s.cfg.Options.Events.CatalogPath)
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		return nil
	}
	creator := nsqmessaging.NewTopicCreator(nsqdAddr, slog.Default())
	if err := creator.EnsureTopics(topics); err != nil {
		return fmt.Errorf("ensure durable NSQ topics: %w", err)
	}
	log.Infow("durable NSQ topics ensured", "topics", topics)
	return nil
}

func durableTopicNamesFromCatalog(catalogPath string) ([]string, error) {
	if catalogPath == "" {
		catalogPath = "configs/events.yaml"
	}
	cfg, err := eventcatalog.Load(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("load event catalog %q for durable topics: %w", catalogPath, err)
	}
	topics := make(map[string]struct{})
	for eventType, eventCfg := range cfg.Events {
		if eventCfg.Delivery != eventcatalog.DeliveryClassDurableOutbox {
			continue
		}
		topicName, ok := cfg.GetTopicName(eventType)
		if !ok {
			return nil, fmt.Errorf("durable event %q has no topic name", eventType)
		}
		topics[topicName] = struct{}{}
	}

	names := make([]string, 0, len(topics))
	for topic := range topics {
		names = append(names, topic)
	}
	sort.Strings(names)
	return names, nil
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
