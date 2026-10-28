package authz

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type inertPlanSender struct{ transaction.Sender }

func (inertPlanSender) Reserve(context.Context, transaction.Request, transaction.FrozenPlan) (transaction.Reservation, error) {
	panic("constructor must not reserve")
}

func TestReadableIAMPlanBindsExactConfiguredNameAndCannotGuessScheme(t *testing.T) {
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: new(sql.DB), SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	original, err := NewTransactionUnitOfWork(db, nil, transaction.Binding{Owner: "iam.mysql", StoreID: "primary", WriterEpoch: "m3-1"})
	require.NoError(t, err)
	const topic = "iam.authz.version.v2"
	readable, err := transport.PhysicalTopicV3(topic, transport.TransactionMessageType, "iam.mysql", "m3-1")
	require.NoError(t, err)
	config := PlanConfig{LogicalTopic: topic, PhysicalTopic: readable, NamingScheme: transport.ReadableNamingV1, Sends: 8, ProofBudget: 2 * time.Second, Admission: func() bool { return true }}
	planned, err := NewPlannedUnitOfWork(original, inertPlanSender{}, config)
	require.NoError(t, err)
	require.Equal(t, readable, planned.config.PhysicalTopic)
	config.NamingScheme = ""
	_, err = NewPlannedUnitOfWork(original, inertPlanSender{}, config)
	require.Error(t, err, "scheme cannot be inferred from a readable resource name")
	config.NamingScheme = transport.ReadableNamingV1
	config.PhysicalTopic, _ = transport.PhysicalTopicV2(topic, transport.TransactionMessageType, "iam.mysql", "m3-1")
	_, err = NewPlannedUnitOfWork(original, inertPlanSender{}, config)
	require.Error(t, err, "readable configuration accepted retired hashed name")
	config.NamingScheme = ""
	_, err = NewPlannedUnitOfWork(original, inertPlanSender{}, config)
	require.NoError(t, err, "explicit v2 configuration must retain old behavior")
}
