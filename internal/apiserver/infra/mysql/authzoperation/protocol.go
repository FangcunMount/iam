package authzoperation

import (
	"context"
	"database/sql"
	"errors"

	operation "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/operation"
	change "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	"github.com/FangcunMount/reliable-messaging/transaction"
)

var _ operation.Reader = (*Store)(nil)
var _ operation.ProtocolReader = (*Store)(nil)

// ReadProtocol reads the pinned schema-v1 receipts in one new authoritative
// statement. No producer/client is created and no final confirmation is sent.
// Broker IDs, wire and execution capabilities never become public query fields.
func (s *Store) ReadProtocol(ctx context.Context, candidate operation.Candidate) (operation.ProtocolState, error) {
	if err := s.validateCandidateRoutes(candidate, false); err != nil {
		return operation.ProtocolState{}, err
	}
	if ctx == nil || !candidate.Valid() || candidate.Binding().Owner != s.binding.Owner || candidate.Binding().StoreID != s.binding.StoreID {
		return operation.ProtocolState{}, operation.ErrInvalid
	}
	if !candidate.Changed() {
		return operation.ProtocolState{Half: transaction.HalfNotAttempted, Local: transaction.LocalNotStarted, Final: transaction.FinalNotAttempted}, nil
	}
	expected, err := candidate.Plan()
	if err != nil {
		return operation.ProtocolState{}, err
	}
	request := transaction.Request{Owner: s.binding.Owner, Scope: change.PolicyScope, Key: candidate.Identity(), InputHash: candidate.InputHash()}
	key, branch := request.KeyDigest(), transaction.BranchKey(change.PolicyBranch)
	var owner, scope, identity string
	var input, rawPlan, rawRef []byte
	var storeID, epoch, executionState, witnessState, half, final sql.NullString
	var schema, fence sql.NullInt64
	var started sql.NullBool
	err = s.db.QueryRowContext(ctx, `SELECT r.owner,r.scope,r.idempotency_key,r.input_hash,IF(OCTET_LENGTH(e.plan)<=2097152,e.plan,NULL),e.writer_epoch,e.fence,e.state,e.prepare_started,w.reference,w.state,w.half_outcome,w.final_outcome,b.store_id,b.schema_version FROM rm_tx_request r LEFT JOIN rm_tx_execution e ON e.execution_key=r.active_execution LEFT JOIN rm_tx_witness w ON w.execution_key=e.execution_key AND w.branch_key=? LEFT JOIN rm_tx_binding b ON b.owner=r.owner WHERE r.request_key=?`, branch[:], key[:]).Scan(&owner, &scope, &identity, &input, &rawPlan, &epoch, &fence, &executionState, &started, &rawRef, &witnessState, &half, &final, &storeID, &schema)
	if errors.Is(err, sql.ErrNoRows) {
		return operation.ProtocolState{Half: transaction.HalfNotAttempted, Local: transaction.LocalNotStarted, Final: transaction.FinalNotAttempted}, nil
	}
	if err != nil {
		return operation.ProtocolState{}, err
	}
	hash := candidate.InputHash()
	if owner != request.Owner || scope != request.Scope || identity != request.Key || string(input) != string(hash[:]) || !storeID.Valid || storeID.String != s.binding.StoreID || !schema.Valid || schema.Int64 != 1 || !epoch.Valid || epoch.String != candidate.Binding().WriterEpoch || !fence.Valid || fence.Int64 < 1 || !executionState.Valid || !started.Valid || !witnessState.Valid || !half.Valid || !final.Valid {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	plan, err := transaction.DecodePlan(rawPlan)
	if err != nil || plan.Digest() != expected.Digest() {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	reference, err := transaction.DecodeReference(rawRef)
	if err != nil {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	want, err := expected.Reference(change.PolicyBranch, uint64(fence.Int64), epoch.String)
	if err != nil || reference != want {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	if executionState.String != witnessState.String {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	result := operation.ProtocolState{Exists: true, Half: transaction.HalfOutcome(half.String), Local: transaction.LocalUnknown, Final: transaction.FinalOutcome(final.String)}
	switch result.Half {
	case transaction.HalfNotAttempted, transaction.HalfPrepared, transaction.HalfRejected, transaction.HalfUnknown:
	default:
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	switch result.Final {
	case transaction.FinalNotAttempted, transaction.FinalCommitConfirmed, transaction.FinalRollbackConfirmed, transaction.FinalUnknown:
	default:
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	switch transaction.State(executionState.String) {
	case transaction.Committed:
		result.Local = transaction.LocalCommitted
	case transaction.Aborted:
		result.Local = transaction.LocalAborted
	case transaction.Prepared:
		if result.Half != transaction.HalfPrepared {
			result.Local = transaction.LocalNotStarted
		}
	default:
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	if (!started.Bool && result.Half != transaction.HalfNotAttempted) || (result.Local == transaction.LocalCommitted && result.Half != transaction.HalfPrepared) || (result.Final == transaction.FinalCommitConfirmed && result.Local != transaction.LocalCommitted) || (result.Final == transaction.FinalRollbackConfirmed && result.Local != transaction.LocalAborted) {
		return operation.ProtocolState{}, operation.ErrProtocolConflict
	}
	// prepare_started is durable before any send. An interrupted RPC before
	// receipt persistence is Unknown, not proof that no attempt ever happened.
	if started.Bool && result.Half == transaction.HalfNotAttempted {
		result.Half = transaction.HalfUnknown
	}
	if started.Bool && result.Final == transaction.FinalNotAttempted && result.Local != transaction.LocalNotStarted && result.Local != transaction.LocalUnknown {
		result.Final = transaction.FinalUnknown
	}
	return result, nil
}
