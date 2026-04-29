package eventcatalog_test

import (
	"testing"

	"github.com/FangcunMount/iam/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
)

const catalogWithoutHandler = `
version: "1"
topics:
  notify:
    name: test.notify
events:
  test.best_effort:
    topic: notify
    delivery: best_effort
    aggregate: Notification
    domain: test
`

func TestParseRequiresHandlerByDefault(t *testing.T) {
	_, err := eventcatalog.Parse([]byte(catalogWithoutHandler))

	require.Error(t, err)
	require.ErrorContains(t, err, "empty handler")
}

func TestParseWithOptionsCanSkipHandlerRequirement(t *testing.T) {
	cfg, err := eventcatalog.ParseWithOptions([]byte(catalogWithoutHandler), eventcatalog.ValidateOptions{})

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NoError(t, cfg.ValidateWithOptions(eventcatalog.ValidateOptions{}))
}

func TestParseWithOptionsStillValidatesTopicAndDelivery(t *testing.T) {
	_, err := eventcatalog.ParseWithOptions([]byte(`
version: "1"
topics:
  notify:
    name: test.notify
events:
  test.best_effort:
    topic: missing
    delivery: best_effort
    aggregate: Notification
    domain: test
`), eventcatalog.ValidateOptions{})

	require.ErrorContains(t, err, "unknown topic")

	_, err = eventcatalog.ParseWithOptions([]byte(`
version: "1"
topics:
  notify:
    name: test.notify
events:
  test.best_effort:
    topic: notify
    delivery: eventually
    aggregate: Notification
    domain: test
`), eventcatalog.ValidateOptions{})

	require.ErrorContains(t, err, "invalid delivery")
}
