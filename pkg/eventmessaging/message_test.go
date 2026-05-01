package eventmessaging_test

import (
	"encoding/json"
	"testing"

	"github.com/FangcunMount/iam/v2/pkg/event"
	"github.com/FangcunMount/iam/v2/pkg/eventmessaging"
	"github.com/stretchr/testify/require"
)

func TestBuildMessageAdaptsEventToComponentBaseMessage(t *testing.T) {
	evt := event.New("test.created", "Test", "test-1", map[string]string{"name": "alice"})

	msg, err := eventmessaging.BuildMessage(evt, "test-source")

	require.NoError(t, err)
	require.Equal(t, evt.EventID(), msg.UUID)
	require.Equal(t, evt.EventType(), msg.Metadata["event_type"])
	require.Equal(t, "test-source", msg.Metadata["source"])

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(msg.Payload, &decoded))
	require.Equal(t, "alice", decoded["name"])
}
