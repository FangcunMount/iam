package roleinheritance

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// RevokeOutcome describes the result of an atomic revoke attempt.
type RevokeOutcome string

const (
	RevokeOutcomeRevoked        RevokeOutcome = "revoked"
	RevokeOutcomeAlreadyRevoked RevokeOutcome = "already_revoked"
	RevokeOutcomeNotFound       RevokeOutcome = "not_found"
)

// AtomicRevoker performs tenant-scoped revoke operations with explicit outcomes.
type AtomicRevoker interface {
	AtomicRevoke(ctx context.Context, id meta.ID, tenantID string) (RevokeOutcome, error)
}

// AppliesVersionChange reports whether a revoke outcome should publish policy versions.
func (o RevokeOutcome) AppliesVersionChange() bool {
	return o == RevokeOutcomeRevoked
}

// IsSuccess reports whether the revoke request completed without a client error.
func (o RevokeOutcome) IsSuccess() bool {
	return o == RevokeOutcomeRevoked
}
