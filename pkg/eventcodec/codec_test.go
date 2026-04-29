package eventcodec_test

import (
	"encoding/json"
	"testing"

	"github.com/FangcunMount/iam/pkg/event"
	"github.com/FangcunMount/iam/pkg/eventcodec"
	"github.com/stretchr/testify/require"
)

func TestEncodePayloadSerializesEventPayload(t *testing.T) {
	evt := event.New("test.created", "Test", "test-1", map[string]any{"name": "alice"})

	payload, err := eventcodec.EncodePayload(evt)

	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, "alice", decoded["name"])
}

func TestMetadataFromEventKeepsStableKeys(t *testing.T) {
	evt := event.New("test.created", "Test", "test-1", map[string]string{"name": "alice"})

	metadata := eventcodec.MetadataFromEvent(evt, "test-source")

	require.Equal(t, evt.EventType(), metadata["event_type"])
	require.Equal(t, evt.AggregateType(), metadata["aggregate_type"])
	require.Equal(t, evt.AggregateID(), metadata["aggregate_id"])
	require.NotEmpty(t, metadata["occurred_at"])
	require.Equal(t, "test-source", metadata["source"])
}
