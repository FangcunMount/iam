package eventcatalog

import sharedcatalog "github.com/FangcunMount/iam/pkg/eventcatalog"

type TopicResolver = sharedcatalog.TopicResolver
type DeliveryClassResolver = sharedcatalog.DeliveryClassResolver
type Catalog = sharedcatalog.Catalog

var NewCatalog = sharedcatalog.NewCatalog
