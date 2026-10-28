package authzoperation

import (
	"database/sql"
	"testing"
	"time"

	operation "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/operation"
	change "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOperationRouteEvidenceIsExactAndHistoricalDoesNotGrantReservation(t *testing.T) {
	const topic = "iam.authz.version.v2"
	old, err := transport.PhysicalTopicV2(topic, "TRANSACTION", "iam.mysql", "m3-1")
	require.NoError(t, err)
	current, err := transport.PhysicalTopicV3(topic, "TRANSACTION", "iam.mysql", "m3-1")
	require.NoError(t, err)
	active := transport.RouteBinding{Topic: topic, PhysicalTopic: current, MessageType: "TRANSACTION", Owner: "iam.mysql", RouteRevision: "m3-1", NamingScheme: transport.ReadableNamingV1}
	history := transport.RouteBinding{Topic: topic, PhysicalTopic: old, MessageType: "TRANSACTION", Owner: "iam.mysql", RouteRevision: "m3-1"}
	routes, err := transport.NewRouteResolver([]transport.RouteBinding{active}, []transport.RouteBinding{history})
	require.NoError(t, err)
	binding := transaction.Binding{Owner: "iam.mysql", StoreID: "primary", WriterEpoch: "m3-1"}
	store, err := NewWithRoutes(new(sql.DB), binding, routes)
	require.NoError(t, err)
	command, err := change.NewCommandIntent("role.create", "user:7", "global", "historical", map[string]string{"name": "test"})
	require.NoError(t, err)
	id := uuid.NewString()
	plan, err := change.FreezeVersionPlan(command, 1, id, uuid.NewString(), time.Now().UTC(), topic, old)
	require.NoError(t, err)
	candidate, err := operation.NewCandidate(command, id, binding, 1, []operation.Permission{{Resource: "iam:authz:collection:roles", Action: "create"}}, &plan)
	require.NoError(t, err)
	original := candidate.Encode()
	require.NoError(t, store.validateCandidateRoutes(candidate, false))
	require.ErrorIs(t, store.validateCandidateRoutes(candidate, true), transaction.ErrBinding)
	_, err = store.Reserve(t.Context(), candidate, time.Minute)
	require.ErrorIs(t, err, transaction.ErrBinding, "historical candidate must not reach an uninitialized SQL pool")
	onlyActive, err := transport.NewRouteResolver([]transport.RouteBinding{active}, nil)
	require.NoError(t, err)
	noHistory, err := NewWithRoutes(new(sql.DB), binding, onlyActive)
	require.NoError(t, err)
	_, err = noHistory.ReadProtocol(t.Context(), candidate)
	require.ErrorIs(t, err, transaction.ErrBinding, "unregistered historical evidence must fail before SQL")
	require.Equal(t, original, candidate.Encode())
	input := plan.Input()
	input.Branches[0].PhysicalTopic = current
	newPlan, err := transaction.NewPlan(input)
	require.NoError(t, err)
	newCandidate, err := operation.NewCandidate(command, id, binding, 1, candidate.Permissions(), &newPlan)
	require.NoError(t, err)
	require.NoError(t, store.validateCandidateRoutes(newCandidate, true))
	require.Equal(t, plan.Input().Branches[0].Wire, newPlan.Input().Branches[0].Wire)
	_, err = NewWithRoutes(new(sql.DB), binding, nil)
	require.ErrorIs(t, err, transaction.ErrBinding)
}
