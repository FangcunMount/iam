package eventcodec_test

import (
	"encoding/json"
	"testing"

	policy "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/FangcunMount/iam/internal/pkg/eventcodec"
	"github.com/stretchr/testify/require"
)

func TestAuthzVersionChangedKeepsLegacyPayloadShape(t *testing.T) {
	evt := policy.NewVersionChangedEvent("tenant-a", 7)

	payload, err := eventcodec.EncodePayload(evt)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, map[string]any{"tenant_id": "tenant-a", "version": float64(7)}, decoded)
}

func TestLoginOTPSMSKeepsLegacyPayloadShapeAndTopicMetadata(t *testing.T) {
	evt := sms.NewLoginOTPSMSEvent("+8613800138000", "123456")

	msg, err := eventcodec.BuildMessage(evt, event.SourceAPIServer)
	require.NoError(t, err)

	var decoded sms.LoginOTPSMSPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &decoded))
	require.Equal(t, sms.EventLoginOTPSMS, decoded.EventType)
	require.Equal(t, "login", decoded.Scene)
	require.Equal(t, "+8613800138000", decoded.PhoneE164)
	require.Equal(t, "123456", decoded.Code)
	require.Equal(t, eventcatalog.LoginOTPSMS, msg.Metadata["event_type"])
}
