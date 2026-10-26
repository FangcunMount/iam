package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	outboxport "github.com/FangcunMount/iam/v5/pkg/outbox"
	"gorm.io/gorm"
)

// EventingDeps holds inputs required to initialize the event platform.
type EventingDeps struct {
	EnableSMSMessaging bool
	Messaging          options.MessagingOptions
	ReliableMessaging  options.ReliableMessagingOptions
	DB                 *gorm.DB
	CatalogPath        string
}

// Eventing holds initialized event platform collaborators.
type Eventing struct {
	Stager          event.Stager
	ReliableRuntime interface {
		Start(context.Context) error
		Stop(context.Context) error
	}
	Catalog   *eventcatalog.Catalog
	Publisher event.Publisher
	Outbox    outboxport.StatusReader
}

// InitEventing assembles only the final IAM transaction owner and optional NORMAL publisher.
func InitEventing(deps EventingDeps) (*Eventing, error) {
	_, finalOwner := deps.Messaging.Transactions["iam.mysql"]
	if deps.Messaging.SelectedProvider() != "rocketmq" || len(deps.Messaging.Transactions) != 1 || !finalOwner || deps.ReliableMessaging.Enabled {
		return nil, fmt.Errorf("final RocketMQ transaction owner required; NSQ and publishing Outbox/Relay are retired: %w", eventoutbox.ErrUnsafeMessagingHandoff)
	}
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

	owner, err := NewTransactionEventing(deps, catalog, "")
	if err != nil {
		return nil, err
	}
	result.Stager = owner.Writer
	result.Publisher, err = owner.DirectPublisher(catalog)
	if err != nil {
		return nil, err
	}
	result.ReliableRuntime = owner
	// Retain the read-only unresolved historical-work check; this is not a publisher.
	result.Outbox = eventoutbox.NewStandardStatusReader(deps.DB)
	return result, nil
}
