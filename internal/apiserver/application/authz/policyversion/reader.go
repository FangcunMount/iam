package policyversion

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
)

type currentRepository interface {
	GetCurrent(context.Context) (*policy.PolicyVersion, error)
}

// Reader reports the committed policy version; it does not consult or update
// the in-memory authorization runtime.
type Reader struct {
	repository currentRepository
}

func NewReader(repository currentRepository) *Reader {
	return &Reader{repository: repository}
}

func (r *Reader) ReadCommittedVersion(ctx context.Context) (int64, error) {
	if r == nil || r.repository == nil {
		return 0, fmt.Errorf("authorization policy version repository unavailable")
	}
	current, err := r.repository.GetCurrent(ctx)
	if err != nil {
		return 0, fmt.Errorf("read committed authorization policy version: %w", err)
	}
	if current == nil || current.Version <= 0 {
		return 0, fmt.Errorf("committed authorization policy version unavailable")
	}
	return current.Version, nil
}
