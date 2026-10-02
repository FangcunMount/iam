package process

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
)

// prepareMessaging rejects retired configuration before creating SDK resources.
func (s *apiServer) prepareMessaging() error {
	if !s.reliableMessagingEnabled() || s.cfg.NSQOptions == nil || !s.cfg.NSQOptions.ConsumerSDKEnabled {
		return fmt.Errorf("legacy messaging runtime is retired; enable standard Outbox and SDK policy subscriber")
	}
	return s.ensureSDKDurableTopics()
}

func (s *apiServer) ensureSDKDurableTopics() error {
	if s == nil || s.cfg == nil || s.cfg.Options == nil || s.cfg.Options.Events == nil || s.cfg.NSQOptions == nil || !s.cfg.NSQOptions.Enabled {
		return fmt.Errorf("SDK durable topic preparation requires NSQ and event configuration")
	}
	topics, err := durableTopicNamesFromCatalog(s.cfg.Options.Events.CatalogPath)
	if err != nil {
		return err
	}
	provisioner, err := sdknsq.NewProvisioner(&http.Client{Timeout: 7 * time.Second}, s.cfg.NSQOptions.NSQdHTTPAddrs)
	if err != nil {
		return fmt.Errorf("configure SDK NSQ topic preparation: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, topic := range topics {
		if err := provisioner.EnsureTopic(ctx, topic); err != nil {
			return fmt.Errorf("prepare durable NSQ topic %q: %w", topic, err)
		}
	}
	log.Infow("SDK durable NSQ topics ensured", "topics", topics)
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
