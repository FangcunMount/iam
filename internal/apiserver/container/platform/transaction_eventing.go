package platform

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	journal "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/authzoperation"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messageprotocol"
	hostuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
)

type normalTransactionPeer interface {
	Start() error
	PublishRaw(context.Context, string, []byte) transport.Result
	Close(context.Context) error
}

// TransactionEventing owns only its protocol and optional NORMAL clients. The
// host keeps its database pool, schema installation and business lifecycle.
type TransactionEventing struct {
	Writer                   *hostuow.PlannedUnitOfWork
	runtime                  *TransactionRuntime
	maintenance              *operationMaintenance
	normal                   normalTransactionPeer
	mu                       sync.Mutex
	started, ready, stopping bool
	startDone                chan struct{}
	cleanupBudget            time.Duration
	preflight                func(context.Context) error
}

// NewTransactionEventing performs no DB/Broker calls, DDL or background work.
// Options read the explicitly supplied local manifest, CA and credential files.
func NewTransactionEventing(deps EventingDeps, catalog *eventcatalog.Catalog, maintenancePrincipal string) (*TransactionEventing, error) {
	if deps.ReliableMessaging.Enabled {
		return nil, errors.New("transaction owner and standard Outbox cannot both be enabled")
	}
	if deps.DB == nil || catalog == nil || catalog.Config() == nil || catalog.Config().Version != "m3-1" || len(deps.Messaging.Transactions) != 1 {
		return nil, errors.New("explicit IAM primary binding and m3-1 transaction catalog required")
	}
	topic, ok := catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	delivery, _ := catalog.GetDeliveryClass(eventing.AuthzVersionChanged)
	if !ok || delivery != eventcatalog.DeliveryClassTransaction {
		return nil, errors.New("policy events require explicit transaction delivery")
	}
	binding, ok := deps.Messaging.Transactions["iam.mysql"]
	if !ok {
		return nil, transaction.ErrBinding
	}
	local, err := hostuow.NewTransactionUnitOfWork(deps.DB, nil, transaction.Binding{Owner: "iam.mysql", StoreID: binding.StoreID, WriterEpoch: binding.WriterEpoch})
	if err != nil {
		return nil, err
	}
	config, err := deps.Messaging.TransactionConfig("iam.mysql", []string{topic})
	if err != nil {
		return nil, err
	}
	if len(config.Routes) != 1 {
		return nil, errors.New("one exact IAM policy transaction route required")
	}
	// The owner budget is shared by SDK evidence and business receipts; adding
	// the journal must not silently double the configured scan allowance.
	if config.Limits.ReconcileBatch < 2 || config.Limits.ReconcileBatch > 100 {
		return nil, errors.New("IAM shared maintenance batch must be between2 and100")
	}
	journalBatch := config.Limits.ReconcileBatch - config.Limits.ReconcileBatch/2
	config.Limits.ReconcileBatch /= 2
	pool, err := deps.DB.DB()
	if err != nil {
		return nil, err
	}
	operations, err := journal.New(pool, local.Store().Binding())
	if err != nil {
		return nil, err
	}
	native, err := NewSQLNativeLimits(pool, local.Store().Binding(), binding.RouteRevision)
	if err != nil {
		return nil, err
	}
	driver, err := rocketmq.NewTransactionalSender(config, local.Store())
	if err != nil {
		return nil, err
	}
	runtime := &TransactionRuntime{sender: driver, admission: deps.Messaging.PublishEnabled}
	if err = attachProtocolMonitor(runtime, local.Store(), config.Limits, native); err != nil {
		return nil, err
	}
	owner := &TransactionEventing{runtime: runtime, startDone: make(chan struct{}), cleanupBudget: config.Limits.RPCTimeout, maintenance: &operationMaintenance{interval: config.Limits.ReconcileInterval, budget: config.Limits.CheckTimeout, batch: journalBatch, abort: operations.AbortExpired}, preflight: func(ctx context.Context) error {
		check, cancel := context.WithTimeout(ctx, config.Limits.CheckTimeout)
		defer cancel()
		if err := messageprotocol.RequireTransaction(check, deps.DB, local.Store().Binding(), "m3-1", false); err != nil {
			return err
		}
		if err := operations.CheckReady(check); err != nil {
			return err
		}
		return checkTransactionHandoff(check, deps.DB, config.Limits.CheckTimeout)
	}}
	owner.Writer, err = hostuow.NewPlannedUnitOfWork(local, runtime, hostuow.PlanConfig{LogicalTopic: topic, PhysicalTopic: config.Routes[0].PhysicalTopic, NamingScheme: config.Routes[0].NamingScheme, Sends: config.Limits.Sends, ProofBudget: config.Limits.CheckTimeout, Admission: owner.admitted, MaintenancePrincipal: maintenancePrincipal, Journal: operations, OperationLease: config.Limits.Lease})
	if err != nil {
		return nil, err
	}
	if deps.EnableSMSMessaging {
		normalTopic, ok := catalog.GetTopicForEvent(eventing.LoginOTPSMS)
		normalDelivery, _ := catalog.GetDeliveryClass(eventing.LoginOTPSMS)
		if !ok || normalDelivery != eventcatalog.DeliveryClassBestEffort {
			return nil, errors.New("enabled MQ SMS requires a NORMAL catalog route")
		}
		normalConfig, err := deps.Messaging.RocketMQ([]string{normalTopic}, config.Limits.Sends, config.Limits.RPCTimeout)
		if err != nil {
			return nil, err
		}
		for _, route := range normalConfig.Routes {
			if route.MessageType != transport.NormalMessageType || route.Owner != "iam.mysql" || route.RouteRevision != "m3-1" {
				return nil, errors.New("SMS direct publishing requires final IAM NORMAL routes")
			}
		}
		owner.normal, err = rocketmq.NewPublisher(normalConfig)
		if err != nil {
			return nil, err
		}
	}
	return owner, nil
}
func (o *TransactionEventing) admitted() bool {
	o.mu.Lock()
	ready := o.ready && !o.stopping
	o.mu.Unlock()
	return ready && o.runtime.Stats().Ready
}
func (o *TransactionEventing) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.started || o.stopping {
		o.mu.Unlock()
		return errors.New("transaction event owner already started or stopped")
	}
	o.started = true
	o.mu.Unlock()
	defer close(o.startDone)
	var err error
	if o.preflight != nil {
		err = o.preflight(ctx)
	}
	if err == nil {
		err = o.runtime.Start(ctx)
	}
	o.mu.Lock()
	interrupted := o.stopping
	o.mu.Unlock()
	if err == nil && !interrupted && o.normal != nil {
		err = o.normal.Start()
	}
	if err == nil && !interrupted && o.maintenance != nil {
		err = o.maintenance.start()
	}
	o.mu.Lock()
	stopped := o.stopping
	if err == nil && !stopped {
		o.ready = true
	}
	o.mu.Unlock()
	if err != nil || stopped {
		cleanup, cancel := context.WithTimeout(context.Background(), o.cleanupBudget)
		defer cancel()
		o.Writer.StopAdmission()
		return errors.Join(err, ErrTransactionAdmissionClosed, o.closeClients(cleanup))
	}
	return nil
}

// StopAdmission is final and synchronous. Admitted host calls may finish while
// Checker and clients stay live until Stop joins them; no client is closed here.
func (o *TransactionEventing) StopAdmission() {
	o.mu.Lock()
	o.stopping = true
	o.ready = false
	o.mu.Unlock()
	o.Writer.StopAdmission()
}
func (o *TransactionEventing) CheckReady(context.Context) error {
	o.mu.Lock()
	ready := o.ready && !o.stopping
	o.mu.Unlock()
	// A temporary write pause is independent of protocol/read availability.
	if !ready || !o.runtime.sender.Stats().Ready {
		return ErrTransactionAdmissionClosed
	}
	return nil
}
func (o *TransactionEventing) Stop(ctx context.Context) error {
	o.StopAdmission()
	o.mu.Lock()
	started := o.started
	o.mu.Unlock()
	if started {
		select {
		case <-o.startDone:
		default:
			select {
			case <-o.startDone:
			case <-ctx.Done():
				select {
				case <-o.startDone:
				default:
					return &transaction.ShutdownIncomplete{RuntimePending: true, Cause: ctx.Err()}
				}
			}
		}
	}
	// Keep Checker live while real host planning/business calls still own results.
	if err := o.Writer.Wait(ctx); err != nil {
		return err
	}
	return o.closeClients(ctx)
}
func (o *TransactionEventing) closeClients(ctx context.Context) error {
	if err := o.maintenance.close(ctx); err != nil {
		return err
	}
	err := o.runtime.Close(ctx)
	if o.normal != nil {
		err = errors.Join(err, o.normal.Close(ctx))
	}
	return err
}
func (o *TransactionEventing) PublishRaw(ctx context.Context, topic string, wire []byte) transport.Result {
	if !o.admitted() || o.normal == nil {
		return transport.Result{Outcome: transport.Rejected}
	}
	return o.normal.PublishRaw(ctx, topic, wire)
}
func (o *TransactionEventing) DirectPublisher(catalog *eventcatalog.Catalog) (*messaging.DirectEventPublisher, error) {
	return messaging.NewDirectEventPublisher(catalog, o, eventing.SourceAPIServer)
}
func (o *TransactionEventing) SetAdmission(open bool) error {
	o.mu.Lock()
	allowed := o.ready && !o.stopping
	o.mu.Unlock()
	if open && !allowed {
		return ErrTransactionAdmissionClosed
	}
	return o.runtime.SetAdmission(open)
}
func (o *TransactionEventing) Stats() rocketmq.TransactionStats {
	stats := o.runtime.Stats()
	o.mu.Lock()
	stats.Ready = stats.Ready && o.ready && !o.stopping
	o.mu.Unlock()
	return stats
}

func (o *TransactionEventing) OperationStats() OperationMaintenanceStats {
	return o.maintenance.snapshot()
}

func (o *TransactionEventing) ProtectionStats() ProtocolProtectionStats {
	return o.runtime.ProtectionStats()
}
func (o *TransactionEventing) ReviewProtection(generation uint64, review string) error {
	o.mu.Lock()
	allowed := o.ready && !o.stopping
	o.mu.Unlock()
	if !allowed {
		return ErrProtocolProtection
	}
	return o.runtime.ReviewProtection(generation, review)
}
