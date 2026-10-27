package authz

import (
	"context"
	"errors"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	change "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	app "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	assignment "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	journal "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/authzoperation"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/policywrite"
	database "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrWriteAdmissionClosed = errors.New("authorization transaction admission closed")

// KnownOperation never grants another execution. T07 maps its public operation
// reference and restores the original typed result after permission checks.
type KnownOperation struct {
	OperationID string
	Local       transaction.LocalOutcome
}

func (*KnownOperation) Error() string {
	return "authorization operation already reserved; inspect original operation"
}

type UnknownOperation struct {
	OperationID string
	Result      transaction.Result
	Cause       error
}

func (*UnknownOperation) Error() string {
	return "authorization transaction outcome requires authoritative operation lookup"
}
func (e *UnknownOperation) Unwrap() error { return e.Cause }

type ReservingSender interface {
	transaction.Sender
	Reserve(context.Context, transaction.Request, transaction.FrozenPlan) (transaction.Reservation, error)
}
type PlanConfig struct {
	LogicalTopic, PhysicalTopic string
	NamingScheme                string
	Routes                      *transport.RouteResolver
	Sends                       int
	ProofBudget                 time.Duration
	Admission                   func() bool
	// Maintenance must explicitly bind a trusted CLI/operator identity. An input
	// ChangedBy string cannot authenticate a caller or select a database owner.
	MaintenancePrincipal string
	Journal              *journal.Store
	OperationLease       time.Duration
}
type PlannedUnitOfWork struct {
	original  *TransactionUnitOfWork
	sender    ReservingSender
	config    PlanConfig
	admission *planAdmission
	routes    *transport.RouteResolver
}

func NewPlannedUnitOfWork(original *TransactionUnitOfWork, sender ReservingSender, config PlanConfig) (*PlannedUnitOfWork, error) {
	expected, err := transport.ExpectedPhysicalTopic(config.LogicalTopic, transport.TransactionMessageType, "iam.mysql", "m3-1", config.NamingScheme)
	if original == nil || original.store == nil || sender == nil || err != nil || expected != config.PhysicalTopic || config.Sends < 1 || config.Sends > 1024 || config.ProofBudget <= 0 || config.Admission == nil {
		return nil, change.ErrInvalidChangePlan
	}
	if config.Journal != nil && (config.OperationLease <= 0 || config.OperationLease > 24*time.Hour) {
		return nil, change.ErrInvalidChangePlan
	}
	routes := config.Routes
	if routes == nil {
		routes, err = transport.NewRouteResolver([]transport.RouteBinding{{Topic: config.LogicalTopic, PhysicalTopic: config.PhysicalTopic, MessageType: transport.TransactionMessageType, Owner: "iam.mysql", RouteRevision: "m3-1", NamingScheme: config.NamingScheme}}, nil)
		if err != nil {
			return nil, err
		}
	}
	active, err := routes.PublishRoute(transport.RouteKey{Topic: config.LogicalTopic, MessageType: transport.TransactionMessageType, Owner: "iam.mysql", RouteRevision: "m3-1"})
	if err != nil || active.PhysicalTopic != config.PhysicalTopic {
		return nil, change.ErrInvalidChangePlan
	}
	return &PlannedUnitOfWork{original: original, sender: sender, config: config, admission: newPlanAdmission(config.Sends), routes: routes}, nil
}
func (*PlannedUnitOfWork) WithinTx(context.Context, func(context.Context, app.TxRepositories) error) error {
	return app.ErrWritePlanRequired
}
func (u *PlannedUnitOfWork) ReadPlan(ctx context.Context, fn func(context.Context, app.TxRepositories) error) error {
	if u == nil || u.original == nil {
		return app.ErrWritePlanRequired
	}
	return u.original.ReadPlan(ctx, fn)
}
func (u *PlannedUnitOfWork) Stage(ctx context.Context, events ...event.DomainEvent) error {
	return (transactionEventStager{}).Stage(ctx, events...)
}
func (u *PlannedUnitOfWork) WithinPlannedTx(ctx context.Context, request app.WriteRequest, fn func(context.Context, app.TxRepositories) error) error {
	principal, ok := management.AuthenticatedPrincipal(ctx)
	if !ok {
		return change.ErrInvalidChangePlan
	}
	return u.within(ctx, principal, request, fn)
}
func (u *PlannedUnitOfWork) WithinPolicyWrite(ctx context.Context, db *gorm.DB, request policywrite.Request, fn func(context.Context) error) error {
	if u == nil || u.original == nil || db != u.original.db || u.config.MaintenancePrincipal == "" || fn == nil {
		return change.ErrInvalidChangePlan
	}
	if _, keyed := change.RequestKey(ctx); keyed {
		return change.ErrInvalidChangePlan
	}
	return u.within(ctx, u.config.MaintenancePrincipal, request.AuthorizationRequest(), func(ctx context.Context, _ app.TxRepositories) error { return fn(ctx) })
}
func (u *PlannedUnitOfWork) within(ctx context.Context, principal string, request app.WriteRequest, fn func(context.Context, app.TxRepositories) error) error {
	if u == nil || u.original == nil || u.admission == nil || u.sender == nil || u.config.Admission == nil || ctx == nil || fn == nil {
		return app.ErrWritePlanRequired
	}
	if _, active := database.TxFromContext(ctx); active {
		return transaction.ErrTransactionRequired
	}
	if !u.config.Admission() {
		return ErrWriteAdmissionClosed
	}
	if err := u.admission.enter(ctx); err != nil {
		return err
	}
	defer u.admission.leave()
	if !u.config.Admission() {
		return ErrWriteAdmissionClosed
	}
	// T07 supplies an optional HTTP/RPC key before planning. Legacy calls receive
	// one operation identity, never cross-request automatic business retries.
	key := uuid.NewString()
	if supplied, ok := change.RequestKey(ctx); ok {
		key = supplied
	}
	command, err := change.NewCommandIntent(request.Operation, principal, request.Scope, key, request.Input)
	if err != nil {
		return err
	}
	if u.config.Journal != nil {
		return u.withBusinessOperation(ctx, command, request, fn)
	}
	lookup := transaction.Request{Owner: "iam.mysql", Scope: change.PolicyScope, Key: command.Key(), InputHash: command.InputHash()}
	existing, err := u.original.store.Lookup(ctx, lookup)
	if err == nil {
		if err = change.MatchCommand(existing.Execution.Plan, command); err != nil {
			return err
		}
		return u.known(existing)
	}
	if !errors.Is(err, transaction.ErrNotFound) {
		return err
	}
	var auditAt time.Time
	preview := request.Preview
	if preview == nil {
		return app.ErrWritePlanRequired
	}
	request.Preview = func(readCtx context.Context, repos app.TxRepositories) (bool, error) {
		changed, err := preview(readCtx, repos)
		if err != nil || !changed {
			return changed, err
		}
		tx, err := database.RequireTx(readCtx)
		if err != nil {
			return false, err
		}
		// Capture the database session's wall clock before half messages; no root
		// business callback may query another clock or generate audit timestamps.
		if err = tx.Statement.ConnPool.QueryRowContext(readCtx, "SELECT CURRENT_TIMESTAMP").Scan(&auditAt); err != nil {
			return false, err
		}
		if auditAt.IsZero() {
			return false, change.ErrInvalidChangePlan
		}
		return true, nil
	}
	snapshot, err := app.PrepareChange(ctx, u.original, request)
	if err != nil {
		return err
	}
	if !snapshot.Changed {
		return nil
	} // durable no-op result is added by T07 before activation
	if err = ctx.Err(); err != nil {
		return err
	}
	builder := change.NewArtifactBuilder(auditAt)
	if request.Freeze != nil {
		if err = request.Freeze(builder); err != nil {
			return err
		}
	}
	artifacts, err := builder.Freeze()
	if err != nil {
		return err
	}
	plan, err := change.FreezeVersionPlanWithArtifacts(command, snapshot.Expected, uuid.NewString(), uuid.NewString(), time.Now().UTC(), u.config.LogicalTopic, u.config.PhysicalTopic, artifacts)
	if err != nil {
		return err
	}
	reserved, err := u.sender.Reserve(ctx, lookup, plan)
	if err != nil {
		return err
	}
	if !reserved.Acquired {
		if err = change.MatchCommand(reserved.Execution.Plan, command); err != nil {
			return err
		}
		return u.known(reserved)
	}
	result, completion, sendErr := messaging.ExecuteJoiningLocal(ctx, u.sender, reserved.Execution, u.original.LocalExecutor(command, fn))
	if completion.Started && completion.Outcome == transaction.LocalCommitted {
		return nil
	} // guarded original root commit ACK, even if final RPC is Unknown
	if result.Local == transaction.LocalCommitted {
		return nil
	}
	if result.Local == transaction.LocalAborted && completion.Err != nil {
		return completion.Err
	}
	return &UnknownOperation{OperationID: reserved.Execution.Plan.Input().OperationID, Result: result, Cause: sendErr}
}
func (u *PlannedUnitOfWork) known(reserved transaction.Reservation) error {
	if err := messaging.ValidateRecordedTransactionRoutes(reserved.Execution.Plan, u.routes); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), u.config.ProofBudget)
	defer cancel()
	refs := reserved.Execution.References()
	proof := make([]*transaction.Evidence, len(refs))
	for i, ref := range refs {
		value, err := u.original.store.Evidence(ctx, ref)
		if err != nil {
			return &KnownOperation{reserved.Execution.Plan.Input().OperationID, transaction.LocalUnknown}
		}
		proof[i] = value
	}
	return &KnownOperation{reserved.Execution.Plan.Input().OperationID, transaction.DecideLocal(refs, proof)}
}

var _ app.UnitOfWork = (*PlannedUnitOfWork)(nil)
var _ app.PlannedUnitOfWork = (*PlannedUnitOfWork)(nil)
var _ policywrite.Writer = (*PlannedUnitOfWork)(nil)

// ApplicationUnitOfWork installs the module's original subject resolver during
// composition, sharing the same owner Store, Sender and admission accounting.
func (u *PlannedUnitOfWork) ApplicationUnitOfWork(resolver assignment.SubjectResolver) app.UnitOfWork {
	original := *u.original
	base := *original.base
	base.subjectResolver = resolver
	original.base = &base
	copy := *u
	copy.original = &original
	return &copy
}
