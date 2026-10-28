package messagefailure

import (
	"database/sql"
	"testing"

	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
)

func TestFailureAuditUsesExactActiveSubscriptionAndCannotInferRetiredTopic(t *testing.T) {
	const topic = "iam.authz.version.v2"
	const sub = "iam.policy-sync.iam-primary-1"
	physical, err := transport.PhysicalTopicV3(topic, transport.TransactionMessageType, "iam.mysql", "m3-1")
	require.NoError(t, err)
	group, err := transport.ConsumerGroupV3(topic, sub)
	require.NoError(t, err)
	row := transport.RouteBinding{Topic: topic, Subscription: sub, PhysicalTopic: physical, ConsumerGroup: group, MessageType: transport.TransactionMessageType, Owner: "iam.mysql", RouteRevision: "m3-1", NamingScheme: transport.ReadableNamingV1}
	// A zero DB cannot issue queries; assembling the audit must remain inert.
	audit, err := NewRocketMQ(new(sql.DB), []transport.RouteBinding{row}, sub)
	require.NoError(t, err)
	m := transport.Received{Topic: topic, Channel: sub, PhysicalTopic: physical}
	require.True(t, audit.permitsRocketMQ(m))
	m.Channel = "qs.authz-sync.apiserver.qs-apiserver-1"
	require.False(t, audit.permitsRocketMQ(m))
	m.Channel = sub
	m.PhysicalTopic, _ = transport.PhysicalTopicV2(topic, transport.TransactionMessageType, "iam.mysql", "m3-1")
	require.False(t, audit.permitsRocketMQ(m), "historical evidence must not authorize current terminal handoff")
	_, err = NewRocketMQ(new(sql.DB), []transport.RouteBinding{row}, "missing")
	require.Error(t, err)
}
