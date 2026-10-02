package platform

import (
	"fmt"
	"strings"
	"time"

	messagingInfra "github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	outboxport "github.com/FangcunMount/iam/v5/pkg/outbox"
	"gorm.io/gorm"
)

// EventingDeps holds inputs required to initialize the event platform.
type EventingDeps struct {
	ReliableMessaging options.ReliableMessagingOptions
	NSQEnabled        bool
	NSQAddress        string
	OutboxInterval    time.Duration
	DB                *gorm.DB
	CatalogPath       string
	OutboxBatch       int
	OutboxRetry       time.Duration
}

// Eventing holds initialized event platform collaborators.
type Eventing struct {
	Stager          event.Stager
	ReliableRuntime *messagingInfra.ReliableRuntime
	Catalog         *eventcatalog.Catalog
	Publisher       event.Publisher
	Outbox          outboxport.StatusReader
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
	result := &Eventing{Catalog: catalog}
	if !deps.ReliableMessaging.Enabled {
		return nil, fmt.Errorf("legacy Outbox runtime is retired; enable events.reliable_messaging and use a fixed prior image for rollback: %w", eventoutbox.ErrUnsafeMessagingHandoff)
	}
	if err := initReliableEventing(deps, result); err != nil {
		return nil, fmt.Errorf("initialize reliable messaging: %w", err)
	}
	return result, nil
}
