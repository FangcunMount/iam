package authzdb

import (
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func Stager(t *testing.T, db *gorm.DB) *eventoutbox.BootstrapStager {
	t.Helper()
	cfg, err := eventcatalog.Parse([]byte(`version: "1"
topics:
  version:
    name: iam.authz.version.v2
events:
  iam.authz.version_changed.v2:
    topic: version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
`))
	require.NoError(t, err)
	stager, err := eventoutbox.NewBootstrapStager(eventcatalog.NewCatalog(cfg))
	require.NoError(t, err)
	return stager
}
