package policyversion

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
)

type currentRepositoryStub struct {
	current *policy.PolicyVersion
	err     error
}

func (s currentRepositoryStub) GetCurrent(context.Context) (*policy.PolicyVersion, error) {
	return s.current, s.err
}

func TestReadCommittedVersion(t *testing.T) {
	reader := NewReader(currentRepositoryStub{current: &policy.PolicyVersion{Version: 42}})
	version, err := reader.ReadCommittedVersion(context.Background())
	if err != nil || version != 42 {
		t.Fatalf("version=%d, err=%v; want 42", version, err)
	}
	for _, repo := range []currentRepositoryStub{
		{},
		{current: &policy.PolicyVersion{Version: 0}},
		{err: errors.New("database unavailable")},
	} {
		version, err := NewReader(repo).ReadCommittedVersion(context.Background())
		if version != 0 || err == nil {
			t.Fatalf("invalid current policy returned version=%d, err=%v", version, err)
		}
	}
}
