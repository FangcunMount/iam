package bootstrapdelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messageprotocol"
	txmysql "github.com/FangcunMount/reliable-messaging/storage/transaction/mysql"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
)

// Sender uses the owner admission gate for all three protocol entrypoints.
type Sender interface {
	transaction.Sender
	Retry(context.Context, transaction.RetryAuthorization) (transaction.Reservation, error)
	Reserve(context.Context, transaction.Request, transaction.FrozenPlan) (transaction.Reservation, error)
}

// Delivery borrows the existing transaction owner, store and physical route.
// Bootstrap authorizations concern only the two immutable migration facts.
type Delivery struct {
	db                        *sql.DB
	store                     *txmysql.Store
	sender                    Sender
	topic, physical, revision string
}

func NewDelivery(db *sql.DB, store *txmysql.Store, sender Sender, topic, physical, revision string) *Delivery {
	return &Delivery{db: db, store: store, sender: sender, topic: topic, physical: physical, revision: revision}
}

// CheckClaim is a read-only startup check. Existing databases without a fresh
// transaction claim have no bootstrap publication work.
func (d *Delivery) CheckClaim(ctx context.Context) (bool, error) {
	var claim, spool bool
	err := d.db.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_fresh_bootstrap_journal'),
		EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_bootstrap_policy_notification')`).Scan(&claim, &spool)
	if err != nil {
		return false, err
	}
	if !claim && !spool {
		return false, nil
	}
	if !claim {
		return false, errors.New("bootstrap notifications have no original claim")
	}
	var count int
	var delivery string
	var identity bool
	var completed uint
	err = d.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM iam_fresh_bootstrap_journal),delivery,
		db_name=CAST(DATABASE() AS BINARY) AND BINARY server_uuid=BINARY @@server_uuid,completed
		FROM iam_fresh_bootstrap_journal WHERE id=1`).Scan(&count, &delivery, &identity, &completed)
	if err != nil {
		return false, err
	}
	if count != 1 || !identity || completed != 35 {
		return false, errors.New("bootstrap claim is incomplete or belongs to another database")
	}
	if delivery == "legacy" && !spool {
		return false, nil
	}
	if delivery != "transaction-v1" || !spool {
		return false, errors.New("bootstrap claim conflicts with notification storage")
	}
	if err := verifyLayout(ctx, d.db); err != nil {
		return false, err
	}
	var records int
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM iam_bootstrap_policy_notification").Scan(&records); err != nil {
		return false, err
	}
	if records != 2 {
		return false, errors.New("expected exactly two bootstrap notifications")
	}
	for _, phase := range []uint{33, 35} {
		if _, err := readNotification(ctx, d.db, d.topic, phase, false); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Advance visits the two migration phases in order. Completion is the local
// delivery state plus the exact SDK commit witness. Broker final receipts and
// consumer ACKs remain separate observations.
func (d *Delivery) Advance(ctx context.Context) (bool, error) {
	for _, phase := range []uint{33, 35} {
		ok, err := d.advancePhase(ctx, phase)
		if err != nil {
			return false, fmt.Errorf("bootstrap notification stage %d: %w", phase, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (d *Delivery) advancePhase(ctx context.Context, phase uint) (bool, error) {
	n, err := readNotification(ctx, d.db, d.topic, phase, false)
	if err != nil {
		return false, err
	}
	plan, err := d.plan(n)
	if err != nil {
		return false, err
	}
	request := transaction.RequestFor(plan)
	reservation, err := d.store.Lookup(ctx, request)
	if errors.Is(err, transaction.ErrNotFound) {
		if n.committed {
			return false, errors.New("bootstrap delivery commit has no request")
		}
		reservation, err = d.sender.Reserve(ctx, request, plan)
	}
	if err != nil {
		return false, err
	}
	if reservation.Execution.Plan.Digest() != plan.Digest() {
		return false, errors.New("bootstrap request differs from the original notification plan")
	}
	if reservation.Acquired {
		return d.execute(ctx, phase, n, reservation.Execution)
	}
	ref, err := plan.Reference("policy", reservation.Execution.Fence, reservation.Execution.WriterEpoch)
	if err != nil {
		return false, err
	}
	evidence, err := d.store.Evidence(ctx, ref)
	if err != nil {
		return false, err
	}
	switch transaction.Resolve(ref, evidence) {
	case transaction.Commit:
		latest, err := readNotification(ctx, d.db, d.topic, phase, false)
		if err != nil {
			return false, err
		}
		if !latest.committed || !bytes.Equal(n.inputBytes, latest.inputBytes) {
			return false, errors.New("bootstrap commit witness conflicts with the original notification")
		}
		return true, nil
	case transaction.Rollback:
		if n.committed {
			return false, errors.New("bootstrap rollback witness conflicts with a committed notification")
		}
		// Keep the failed fences and cap automatic retries of one immutable fact.
		if reservation.Execution.Fence >= 3 {
			return false, errors.New("bootstrap notification retry limit reached")
		}
		retry, err := d.sender.Retry(ctx, transaction.RetryAuthorization{Previous: reservation.Execution})
		if err != nil {
			return false, err
		}
		if !retry.Acquired || retry.Execution.Plan.Digest() != plan.Digest() {
			return false, transaction.ErrStaleExecution
		}
		return d.execute(ctx, phase, n, retry.Execution)
	default:
		// A prepared, missing or contradictory witness cannot authorize a resend.
		return false, nil
	}
}

func (d *Delivery) plan(n notification) (transaction.FrozenPlan, error) {
	in := n.message.Input()
	wire, err := legacy.Encode(legacy.Envelope{UUID: in.ID, Payload: in.Payload, Metadata: eventing.PolicyVersionWireMetadata(n.version)}, legacy.Revision1)
	if err != nil {
		return transaction.FrozenPlan{}, err
	}
	identity := "bootstrap-delivery:" + in.ID
	return transaction.NewPlan(transaction.PlanInput{Owner: d.store.Binding().Owner, Scope: in.Scope, OperationID: identity,
		IdempotencyKey: identity, InputHash: sha256.Sum256(n.inputBytes), Branches: []transaction.BranchInput{
			{ID: "policy", PhysicalTopic: d.physical, RouteRevision: d.revision, Message: n.message, Wire: wire},
		}})
}

func (d *Delivery) execute(ctx context.Context, phase uint, n notification, execution transaction.Execution) (bool, error) {
	_, err := d.sender.Execute(ctx, execution, func(ctx context.Context, e transaction.Execution) (transaction.LocalOutcome, error) {
		return d.commit(ctx, phase, n, e)
	})
	// Always establish completion by a fresh read on a subsequent sweep.
	return false, err
}

func (d *Delivery) commit(ctx context.Context, phase uint, n notification, e transaction.Execution) (transaction.LocalOutcome, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return transaction.LocalNotStarted, err
	}
	defer tx.Rollback()
	guard, err := d.store.Bind(ctx, tx, e)
	if err != nil {
		return transaction.LocalAborted, err
	}
	if err := messageprotocol.RequireTransactionRoot(ctx, tx, d.store.Binding(), d.revision); err != nil {
		return transaction.LocalAborted, err
	}
	current, err := readNotification(ctx, tx, d.topic, phase, true)
	if err != nil {
		return transaction.LocalAborted, err
	}
	if current.committed || !bytes.Equal(current.inputBytes, n.inputBytes) {
		return transaction.LocalAborted, errors.New("bootstrap notification changed before delivery commit")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE iam_bootstrap_policy_notification SET delivery_committed=1 WHERE phase=?`, phase); err != nil {
		return transaction.LocalAborted, err
	}
	if err = guard.CommitWitness(ctx); err != nil {
		return transaction.LocalAborted, err
	}
	if err = tx.Commit(); err != nil {
		return transaction.LocalUnknown, err
	}
	return transaction.LocalCommitted, nil
}
