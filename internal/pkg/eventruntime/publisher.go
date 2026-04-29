package eventruntime

import (
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	sharedruntime "github.com/FangcunMount/iam/pkg/eventruntime"
)

type PublishMode = sharedruntime.PublishMode

const (
	PublishModeMQ      = sharedruntime.PublishModeMQ
	PublishModeLogging = sharedruntime.PublishModeLogging
	PublishModeNop     = sharedruntime.PublishModeNop
)

type RoutingPublisher = sharedruntime.RoutingPublisher
type RoutingPublisherOptions = sharedruntime.RoutingPublisherOptions

var NewRoutingPublisher = sharedruntime.NewRoutingPublisher

func NewPublisherForBus(catalog *eventcatalog.Catalog, bus messaging.EventBus) event.Publisher {
	return sharedruntime.NewPublisherForBus(catalog, bus, event.SourceAPIServer)
}
