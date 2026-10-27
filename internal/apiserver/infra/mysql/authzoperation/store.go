// Package authzoperation persists business idempotency/results separately from
// SDK protocol evidence. No method sends messages, repeats business or installs
// DDL. The host explicitly installs schema44 and owns all connections/roots.
package authzoperation

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"
	"unicode/utf8"

	operation "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/operation"
	change "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/messaging"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	driver "github.com/go-sql-driver/mysql"
)

type State = operation.State

const (
	Prepared  = operation.Prepared
	Committed = operation.Committed
	Aborted   = operation.Aborted
)
const maxResultBytes = operation.MaxResultBytes

type Store struct {
	db      *sql.DB
	binding transaction.Binding
	routes  *transport.RouteResolver
}

// Claim is a one-winner local execution capability. Its token and frozen private
// plan cannot be serialized as public operation status. A known request never
// receives another Claim, including after lease expiration or process exit.
type Claim struct {
	candidate operation.Candidate
	token     [32]byte
	used      atomic.Bool
}
type Reservation struct {
	Snapshot Snapshot
	Claim    *Claim
}
type Snapshot = operation.Record

func New(db *sql.DB, binding transaction.Binding) (*Store, error) {
	if db == nil || !operation.ValidBinding(binding) {
		return nil, operation.ErrInvalid
	}
	return &Store{db: db, binding: binding}, nil
}

// NewWithRoutes binds runtime receipt and operation reads to exact active/history
// evidence. New remains the explicit lower-level storage compatibility adapter.
func NewWithRoutes(db *sql.DB, binding transaction.Binding, routes *transport.RouteResolver) (*Store, error) {
	store, err := New(db, binding)
	if err != nil {
		return nil, err
	}
	if routes == nil {
		return nil, transaction.ErrBinding
	}
	store.routes = routes
	return store, nil
}

func (s *Store) validateCandidateRoutes(candidate operation.Candidate, active bool) error {
	if s.routes == nil || !candidate.Changed() {
		return nil
	}
	plan, err := candidate.Plan()
	if err != nil {
		return err
	}
	if active {
		return messaging.ValidateActiveTransactionRoutes(plan, s.routes)
	}
	return messaging.ValidateRecordedTransactionRoutes(plan, s.routes)
}

const columns = `candidate,candidate_hash,owner,store_id,writer_epoch,identity_key,input_hash,operation_id,request_key,state,policy_version,result,result_hash,guard_version,schema_version,DATE_FORMAT(lease_until,'%Y-%m-%d %H:%i:%s.%f'),DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s.%f'),DATE_FORMAT(finalized_at,'%Y-%m-%d %H:%i:%s.%f')`

func (s *Store) scan(row interface{ Scan(...any) error }) (Snapshot, error) {
	var snapshot Snapshot
	var raw, hash, identity, inputHash, requestKey, resultHash []byte
	var owner, storeID, epoch, id string
	var version sql.NullInt64
	var lease, created string
	var finalized sql.NullString
	var schema int
	if err := row.Scan(&raw, &hash, &owner, &storeID, &epoch, &identity, &inputHash, &id, &requestKey, &snapshot.State, &version, &snapshot.Result, &resultHash, &snapshot.GuardVersion, &schema, &lease, &created, &finalized); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snapshot, operation.ErrNotFound
		}
		return snapshot, err
	}
	candidate, err := operation.DecodeCandidate(raw)
	fingerprint := candidate.Fingerprint()
	key := candidate.RequestKey()
	input := candidate.InputHash()
	if err != nil || !bytes.Equal(hash, fingerprint[:]) || !bytes.Equal(requestKey, key[:]) || !bytes.Equal(inputHash, input[:]) || string(identity) != candidate.Identity() || id != candidate.ID() || owner != candidate.Binding().Owner || storeID != candidate.Binding().StoreID || epoch != candidate.Binding().WriterEpoch || owner != s.binding.Owner || storeID != s.binding.StoreID || schema != 1 || snapshot.GuardVersion < 0 {
		return Snapshot{}, operation.ErrInvalid
	}
	snapshot.LeaseUntil, err = time.ParseInLocation("2006-01-02 15:04:05.000000", lease, time.UTC)
	if err != nil || snapshot.LeaseUntil.IsZero() {
		return Snapshot{}, operation.ErrInvalid
	}
	snapshot.CreatedAt, err = time.ParseInLocation("2006-01-02 15:04:05.000000", created, time.UTC)
	if err != nil || snapshot.CreatedAt.IsZero() {
		return Snapshot{}, operation.ErrInvalid
	}
	if finalized.Valid {
		snapshot.FinalizedAt, err = time.ParseInLocation("2006-01-02 15:04:05.000000", finalized.String, time.UTC)
		if err != nil || snapshot.FinalizedAt.IsZero() {
			return Snapshot{}, operation.ErrInvalid
		}
	}
	if err := s.validateCandidateRoutes(candidate, false); err != nil {
		return Snapshot{}, err
	}
	snapshot.Candidate = candidate
	switch snapshot.State {
	case Prepared, Aborted:
		if version.Valid || len(snapshot.Result) > 0 || len(resultHash) > 0 || ((snapshot.State == Aborted) != finalized.Valid) {
			return Snapshot{}, operation.ErrInvalid
		}
	case Committed:
		hash := sha256.Sum256(snapshot.Result)
		expected := candidate.ExpectedVersion()
		if candidate.Changed() {
			expected++
		}
		if !finalized.Valid || !version.Valid || version.Int64 != expected || !validResult(snapshot.Result) || !bytes.Equal(resultHash, hash[:]) {
			return Snapshot{}, operation.ErrInvalid
		}
		if !candidate.Changed() && !bytes.Equal(snapshot.Result, candidate.NoopResult()) {
			return Snapshot{}, operation.ErrInvalid
		}
		snapshot.PolicyVersion = version.Int64
	default:
		return Snapshot{}, operation.ErrInvalid
	}
	snapshot.Result = bytes.Clone(snapshot.Result)
	return snapshot, nil
}
func validResult(raw []byte) bool {
	return len(raw) > 0 && len(raw) <= maxResultBytes && utf8.Valid(raw) && json.Valid(raw)
}
func (s *Store) Lookup(ctx context.Context, command change.CommandIntent) (Snapshot, error) {
	if ctx == nil {
		return Snapshot{}, operation.ErrInvalid
	}
	key, err := operation.RequestKeyFor(command)
	if err != nil {
		return Snapshot{}, err
	}
	row, err := s.scan(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE request_key=?", key[:]))
	if err != nil {
		return Snapshot{}, err
	}
	if row.Candidate.Identity() != command.Key() || row.Candidate.InputHash() != command.InputHash() {
		return Snapshot{}, operation.ErrConflict
	}
	return row, nil
}
func (s *Store) Read(ctx context.Context, id string) (Snapshot, error) {
	if ctx == nil || !operation.ValidID(id) {
		return Snapshot{}, operation.ErrInvalid
	}
	return s.scan(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE operation_id=?", id))
}
func (s *Store) Reserve(ctx context.Context, candidate operation.Candidate, lease time.Duration) (Reservation, error) {
	if err := s.validateCandidateRoutes(candidate, true); err != nil {
		return Reservation{}, err
	}
	if ctx == nil || !candidate.Valid() || candidate.Binding() != s.binding || lease < time.Microsecond || lease > 24*time.Hour {
		return Reservation{}, operation.ErrInvalid
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return Reservation{}, err
	}
	key, hash, input := candidate.RequestKey(), candidate.Fingerprint(), candidate.InputHash()
	_, err := s.db.ExecContext(ctx, `INSERT INTO iam_authz_operations(request_key,operation_id,owner,store_id,writer_epoch,identity_key,input_hash,candidate,candidate_hash,execution_token,state,lease_until,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,'prepared',TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(6)),UTC_TIMESTAMP(6))`, key[:], candidate.ID(), s.binding.Owner, s.binding.StoreID, s.binding.WriterEpoch, []byte(candidate.Identity()), input[:], candidate.Encode(), hash[:], token[:], lease.Microseconds())
	if err == nil {
		snapshot, readErr := s.scan(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE request_key=?", key[:]))
		if readErr != nil {
			return Reservation{}, readErr
		}
		if snapshot.Candidate.Fingerprint() != candidate.Fingerprint() {
			return Reservation{}, operation.ErrInvalid
		}
		if snapshot.State != Prepared {
			return Reservation{Snapshot: snapshot}, nil
		}
		return Reservation{Snapshot: snapshot, Claim: &Claim{candidate: candidate, token: token}}, nil
	}
	var duplicate *driver.MySQLError
	if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
		return Reservation{}, err
	}
	winner, readErr := s.scan(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE request_key=?", key[:]))
	if readErr != nil {
		if errors.Is(readErr, operation.ErrNotFound) {
			return Reservation{}, operation.ErrInvalid
		}
		return Reservation{}, readErr
	}
	if !winner.Candidate.SameRequest(candidate) {
		return Reservation{}, operation.ErrConflict
	}
	return Reservation{Snapshot: winner}, nil
}

type Guard struct {
	tx        *sql.Tx
	candidate operation.Candidate
	token     [32]byte
	done      bool
}

// Bind borrows the actual original SQL root. It never begins/commits a root or
// selects another connection. Its conditional write competes with the expiry
// aborter; a past lease alone is never a rollback proof. Changed operations must
// bind the SDK guard first in this same root, then bind this business claim.
func (s *Store) Bind(ctx context.Context, tx *sql.Tx, claim *Claim) (*Guard, error) {
	if ctx == nil || tx == nil || claim == nil || !claim.candidate.Valid() || claim.candidate.Binding() != s.binding || claim.token == ([32]byte{}) {
		return nil, operation.ErrStale
	}
	if !claim.used.CompareAndSwap(false, true) {
		return nil, operation.ErrStale
	}
	key, hash := claim.candidate.RequestKey(), claim.candidate.Fingerprint()
	result, err := tx.ExecContext(ctx, `UPDATE iam_authz_operations SET guard_version=guard_version+1 WHERE request_key=? AND operation_id=? AND candidate_hash=? AND execution_token=? AND owner=? AND store_id=? AND writer_epoch=? AND state='prepared' AND schema_version=1 AND guard_version<9223372036854775807 AND lease_until>UTC_TIMESTAMP(6)`, key[:], claim.candidate.ID(), hash[:], claim.token[:], s.binding.Owner, s.binding.StoreID, s.binding.WriterEpoch)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows != 1 {
		return nil, operation.ErrStale
	}
	snapshot, err := s.scan(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE request_key=?", key[:]))
	if err != nil || snapshot.Candidate.Fingerprint() != claim.candidate.Fingerprint() {
		return nil, errors.Join(operation.ErrInvalid, err)
	}
	return &Guard{tx: tx, candidate: claim.candidate, token: claim.token}, nil
}

// Commit records a frozen typed business result, never a Broker confirmation.
// It becomes authoritative only when a new reader sees the original root commit.
func (g *Guard) Commit(ctx context.Context, result []byte, policyVersion int64) error {
	if ctx == nil || g == nil || g.done || !validResult(result) {
		return operation.ErrInvalid
	}
	expected := g.candidate.ExpectedVersion()
	if g.candidate.Changed() {
		expected++
	}
	if policyVersion != expected {
		return operation.ErrInvalid
	}
	if !g.candidate.Changed() && !bytes.Equal(result, g.candidate.NoopResult()) {
		return operation.ErrInvalid
	}
	key := g.candidate.RequestKey()
	hash := sha256.Sum256(result)
	changed, err := g.tx.ExecContext(ctx, `UPDATE iam_authz_operations SET state='committed',policy_version=?,result=?,result_hash=?,finalized_at=UTC_TIMESTAMP(6) WHERE request_key=? AND execution_token=? AND state='prepared' AND schema_version=1`, policyVersion, result, hash[:], key[:], g.token[:])
	if err != nil {
		return err
	}
	rows, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return operation.ErrStale
	}
	g.done = true
	return nil
}

// AbortExpired only fences expired, uncommitted business claims. It does not
// resend their frozen plans, invoke executors or decide Broker transactions.
func (s *Store) AbortExpired(ctx context.Context, limit int) (int, error) {
	if ctx == nil || limit < 1 || limit > 100 {
		return 0, operation.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT request_key FROM iam_authz_operations WHERE owner=? AND store_id=? AND state='prepared' AND lease_until<=UTC_TIMESTAMP(6) ORDER BY lease_until,request_key LIMIT ? FOR UPDATE SKIP LOCKED`, s.binding.Owner, s.binding.StoreID, limit)
	if err != nil {
		return 0, err
	}
	keys := [][]byte{}
	for rows.Next() {
		var key []byte
		if err = rows.Scan(&key); err != nil {
			return 0, errors.Join(err, rows.Close())
		}
		keys = append(keys, bytes.Clone(key))
	}
	readErr := rows.Err()
	closeErr := rows.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		return 0, err
	}
	for _, key := range keys {
		if _, err = s.scan(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE request_key=?", key)); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE iam_authz_operations SET state='aborted',finalized_at=UTC_TIMESTAMP(6) WHERE request_key=? AND state='prepared' AND lease_until<=UTC_TIMESTAMP(6)`, key); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return len(keys), nil
}

// Abort competes with the original root using the winner's exact capability.
// It can observe a concurrent commit instead of aborting it. Neither an error
// nor cancellation is itself proof; callers only use the returned durable state.
func (s *Store) Abort(ctx context.Context, claim *Claim) (Snapshot, error) {
	if ctx == nil || claim == nil || !claim.candidate.Valid() || claim.candidate.Binding() != s.binding || claim.token == ([32]byte{}) {
		return Snapshot{}, operation.ErrStale
	}
	claim.used.Store(true)
	key, hash := claim.candidate.RequestKey(), claim.candidate.Fingerprint()
	_, err := s.db.ExecContext(ctx, `UPDATE iam_authz_operations SET state='aborted',finalized_at=UTC_TIMESTAMP(6) WHERE request_key=? AND operation_id=? AND candidate_hash=? AND execution_token=? AND owner=? AND store_id=? AND writer_epoch=? AND state='prepared' AND schema_version=1`, key[:], claim.candidate.ID(), hash[:], claim.token[:], s.binding.Owner, s.binding.StoreID, s.binding.WriterEpoch)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := s.Read(ctx, claim.candidate.ID())
	if err != nil {
		return Snapshot{}, err
	}
	if snapshot.Candidate.Fingerprint() != claim.candidate.Fingerprint() || (snapshot.State != Aborted && snapshot.State != Committed) {
		return Snapshot{}, operation.ErrInvalid
	}
	return snapshot, nil
}

// CheckReady validates the explicitly installed host table without DDL or
// admitting a business operation. Binding validation remains the SDK's Start.
func (s *Store) CheckReady(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT "+columns+" FROM iam_authz_operations WHERE 1=0")
	if err != nil {
		return err
	}
	return errors.Join(rows.Err(), rows.Close())
}
