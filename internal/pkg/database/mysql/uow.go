package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FangcunMount/component-base/pkg/log"
	"gorm.io/gorm"
)

var (
	// ErrUnitOfWorkUnavailable indicates that transactional execution was requested
	// without a configured database handle.
	ErrUnitOfWorkUnavailable = errors.New("mysql unit of work unavailable")
	// ErrActiveTransactionRequired indicates that an operation requires an active
	// transaction context.
	ErrActiveTransactionRequired = errors.New("mysql active transaction required")
)

// TxOptions describes the database transaction requested by an application use case.
type TxOptions struct {
	Name      string
	ReadOnly  bool
	Isolation sql.IsolationLevel
}

type txContextKey struct{}

type txState struct {
	tx          *gorm.DB
	afterCommit []func(context.Context) error
}

// UnitOfWork wraps a GORM DB to offer transactional execution helpers.
type UnitOfWork struct {
	db *gorm.DB
}

// NewUnitOfWork constructs a UnitOfWork for the given *gorm.DB.
func NewUnitOfWork(db *gorm.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// TxFromContext extracts the active transaction from ctx.
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	state, ok := txStateFromContext(ctx)
	if !ok || state.tx == nil {
		return nil, false
	}
	return state.tx, true
}

// RequireTx extracts the active transaction or returns a structured error.
func RequireTx(ctx context.Context) (*gorm.DB, error) {
	tx, ok := TxFromContext(ctx)
	if !ok {
		return nil, ErrActiveTransactionRequired
	}
	return tx, nil
}

// AfterCommit registers a best-effort hook that runs only after the outermost
// transaction commits successfully.
func AfterCommit(ctx context.Context, hook func(context.Context) error) error {
	if hook == nil {
		return nil
	}
	state, ok := txStateFromContext(ctx)
	if !ok || state.tx == nil {
		return ErrActiveTransactionRequired
	}
	state.afterCommit = append(state.afterCommit, hook)
	return nil
}

// WithinTransaction executes fn inside a database transaction and injects the
// transaction handle into the callback context. If ctx already carries a
// transaction, Required propagation reuses it instead of opening a nested
// transaction.
func (u *UnitOfWork) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error, opts ...TxOptions) error {
	if fn == nil {
		return nil
	}
	if u == nil || u.db == nil {
		return ErrUnitOfWorkUnavailable
	}
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	txOpts := toSQLTxOptions(opts...)
	state := &txState{}
	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state.tx = tx
		txCtx := context.WithValue(ctx, txContextKey{}, state)
		return fn(txCtx)
	}, txOpts)
	if err != nil {
		return err
	}
	for _, hook := range state.afterCommit {
		if hookErr := hook(ctx); hookErr != nil {
			log.Warnf("after commit hook failed: %v", hookErr)
		}
	}
	return nil
}

func txStateFromContext(ctx context.Context) (*txState, bool) {
	if ctx == nil {
		return nil, false
	}
	state, ok := ctx.Value(txContextKey{}).(*txState)
	return state, ok && state != nil
}

func toSQLTxOptions(opts ...TxOptions) *sql.TxOptions {
	if len(opts) == 0 {
		return nil
	}
	opt := opts[0]
	if !opt.ReadOnly && opt.Isolation == sql.LevelDefault {
		return nil
	}
	return &sql.TxOptions{
		Isolation: opt.Isolation,
		ReadOnly:  opt.ReadOnly,
	}
}
