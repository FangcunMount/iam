package authzdb

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func Stager(t *testing.T, db *gorm.DB) *eventoutbox.Store {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`version: "1"
topics:
  version:
    name: iam.authz.version
events:
  iam.authz.version_changed:
    topic: version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
`))
	require.NoError(t, err)
	return eventoutbox.NewStore(db, eventcatalog.NewCatalog(cfg))
}
