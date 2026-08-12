package platform

import (
	"fmt"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
	messagingInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/messaging"
	eventoutbox "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v3/pkg/eventruntime"
	"gorm.io/gorm"
)

// EventingDeps holds inputs required to initialize the event platform.
type EventingDeps struct {
	DB          *gorm.DB
	EventBus    messaging.EventBus
	CatalogPath string
	OutboxBatch int
	OutboxRetry time.Duration
}

// Eventing holds initialized event platform collaborators.
type Eventing struct {
	Catalog   *eventcatalog.Catalog
	Publisher event.Publisher
	Outbox    *eventoutbox.Store
	Relay     messagingInfra.OutboxRelay
}

// InitEventing loads the catalog, publisher, and optional outbox relay.
func InitEventing(deps EventingDeps) (*Eventing, error) {
	catalogPath := strings.TrimSpace(deps.CatalogPath)
	if catalogPath == "" {
		catalogPath = "configs/events.yaml"
	}
	cfg, err := eventcatalog.Load(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("load event catalog %q: %w", catalogPath, err)
	}
	catalog := eventcatalog.NewCatalog(cfg)
	result := &Eventing{
		Catalog:   catalog,
		Publisher: eventruntime.NewPublisherForBus(catalog, deps.EventBus, eventing.SourceAPIServer),
	}
	if deps.DB == nil {
		return result, nil
	}
	result.Outbox = eventoutbox.NewStore(deps.DB, catalog)
	if deps.EventBus == nil {
		log.Warnw("event outbox relay not started: event bus unavailable", "store", "iam.domain_event_outbox")
		return result, nil
	}
	result.Relay = messagingInfra.NewOutboxRelay("iam.domain_event_outbox", result.Outbox, deps.EventBus, messagingInfra.OutboxRelayOptions{
		BatchSize:  deps.OutboxBatch,
		RetryDelay: deps.OutboxRetry,
	})
	return result, nil
}
