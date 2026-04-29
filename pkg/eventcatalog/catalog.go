package eventcatalog

type TopicResolver interface {
	GetTopicForEvent(eventType string) (string, bool)
}

type DeliveryClassResolver interface {
	GetDeliveryClass(eventType string) (DeliveryClass, bool)
}

type Catalog struct {
	config       *Config
	eventToTopic map[string]string
}

func NewCatalog(cfg *Config) *Catalog {
	c := &Catalog{
		config:       cfg,
		eventToTopic: map[string]string{},
	}
	if cfg == nil {
		return c
	}
	for eventType := range cfg.Events {
		if topic, ok := cfg.GetTopicName(eventType); ok {
			c.eventToTopic[eventType] = topic
		}
	}
	return c
}

func (c *Catalog) Config() *Config {
	if c == nil {
		return nil
	}
	return c.config
}

func (c *Catalog) GetTopicForEvent(eventType string) (string, bool) {
	if c == nil {
		return "", false
	}
	topic, ok := c.eventToTopic[eventType]
	return topic, ok
}

func (c *Catalog) GetDeliveryClass(eventType string) (DeliveryClass, bool) {
	if c == nil || c.config == nil {
		return "", false
	}
	return c.config.GetDeliveryClass(eventType)
}

func (c *Catalog) IsDurableOutbox(eventType string) bool {
	delivery, ok := c.GetDeliveryClass(eventType)
	return ok && delivery == DeliveryClassDurableOutbox
}

func (c *Catalog) IsEventRegistered(eventType string) bool {
	_, ok := c.GetTopicForEvent(eventType)
	return ok
}
