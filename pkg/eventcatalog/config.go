package eventcatalog

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DeliveryClass string

const (
	DeliveryClassBestEffort    DeliveryClass = "best_effort"
	DeliveryClassDurableOutbox DeliveryClass = "durable_outbox"
)

func (c DeliveryClass) Valid() bool {
	switch c {
	case DeliveryClassBestEffort, DeliveryClassDurableOutbox:
		return true
	default:
		return false
	}
}

type Config struct {
	Version string                 `yaml:"version"`
	Topics  map[string]TopicConfig `yaml:"topics"`
	Events  map[string]EventConfig `yaml:"events"`
}

// ValidateOptions controls which catalog rules are enforced by shared callers.
type ValidateOptions struct {
	RequireHandler bool
}

type TopicConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type EventConfig struct {
	Topic       string        `yaml:"topic"`
	Delivery    DeliveryClass `yaml:"delivery"`
	Aggregate   string        `yaml:"aggregate"`
	Domain      string        `yaml:"domain"`
	Description string        `yaml:"description"`
	Handler     string        `yaml:"handler"`
}

func Load(path string) (*Config, error) {
	return LoadWithOptions(path, ValidateOptions{RequireHandler: true})
}

func LoadWithOptions(path string, opts ValidateOptions) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read event catalog: %w", err)
	}
	return ParseWithOptions(data, opts)
}

func Parse(data []byte) (*Config, error) {
	return ParseWithOptions(data, ValidateOptions{RequireHandler: true})
}

func ParseWithOptions(data []byte, opts ValidateOptions) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse event catalog: %w", err)
	}
	if err := cfg.ValidateWithOptions(opts); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	return c.ValidateWithOptions(ValidateOptions{RequireHandler: true})
}

func (c *Config) ValidateWithOptions(opts ValidateOptions) error {
	if c == nil {
		return fmt.Errorf("event catalog is nil")
	}
	for eventType, eventCfg := range c.Events {
		topic, ok := c.Topics[eventCfg.Topic]
		if !ok || topic.Name == "" {
			return fmt.Errorf("event %q references unknown topic %q", eventType, eventCfg.Topic)
		}
		if eventCfg.Delivery == "" || !eventCfg.Delivery.Valid() {
			return fmt.Errorf("event %q has invalid delivery %q", eventType, eventCfg.Delivery)
		}
		if opts.RequireHandler && eventCfg.Handler == "" {
			return fmt.Errorf("event %q has empty handler", eventType)
		}
	}
	return nil
}

func (c *Config) GetTopicName(eventType string) (string, bool) {
	if c == nil {
		return "", false
	}
	eventCfg, ok := c.Events[eventType]
	if !ok {
		return "", false
	}
	topicCfg, ok := c.Topics[eventCfg.Topic]
	if !ok {
		return "", false
	}
	return topicCfg.Name, true
}

func (c *Config) GetDeliveryClass(eventType string) (DeliveryClass, bool) {
	if c == nil {
		return "", false
	}
	eventCfg, ok := c.Events[eventType]
	if !ok {
		return "", false
	}
	return eventCfg.Delivery, true
}
