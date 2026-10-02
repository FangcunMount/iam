package eventing_test

import (
	"encoding/json"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
	"github.com/stretchr/testify/require"
)

func TestAuthzVersionChangedPublishesOnlyGlobalVersion(t *testing.T) {
	evt := policy.NewVersionChangedEvent(7)

	payload, err := eventcodec.EncodePayload(evt)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, map[string]any{"version": float64(7)}, decoded)
	require.Equal(t, eventing.AuthzVersionChanged, evt.EventType())
}

func TestLoginOTPSMSKeepsLegacyPayloadShapeAndTopicMetadata(t *testing.T) {
	evt := sms.NewLoginOTPSMSEvent("+8613800138000", "123456")

	payload, err := eventcodec.EncodePayload(evt)
	require.NoError(t, err)

	var decoded sms.LoginOTPSMSPayload
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, sms.EventLoginOTPSMS, decoded.EventType)
	require.Equal(t, "login", decoded.Scene)
	require.Equal(t, "+8613800138000", decoded.PhoneE164)
	require.Equal(t, "123456", decoded.Code)
	require.Equal(t, eventing.LoginOTPSMS, eventcodec.MetadataFromEvent(evt, eventing.SourceAPIServer)["event_type"])
	require.Equal(t, eventing.SourceAPIServer, eventcodec.MetadataFromEvent(evt, eventing.SourceAPIServer)["source"])
}
