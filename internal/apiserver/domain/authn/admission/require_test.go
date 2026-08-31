package admission

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRequireReturnsTypedAdmissionErrors(t *testing.T) {
	t.Parallel()

	subject := Subject{UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2)}
	tests := []struct {
		name       string
		policy     Policy
		wantDenied bool
		wantEval   bool
	}{
		{name: "admitted", policy: requirePolicyStub{decision: Admit(subject)}},
		{name: "denied", policy: requirePolicyStub{decision: Deny(subject, ReasonUserBlocked)}, wantDenied: true},
		{name: "evaluation failed", policy: requirePolicyStub{err: errors.New("status unavailable")}, wantEval: true},
		{name: "missing policy", wantEval: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Require(context.Background(), tt.policy, subject)
			if !tt.wantDenied && !tt.wantEval {
				require.NoError(t, err)
				return
			}
			if tt.wantDenied {
				var denied *DeniedError
				require.ErrorAs(t, err, &denied)
				require.Equal(t, ReasonUserBlocked, denied.Decision.Reason)
			}
			if tt.wantEval {
				var evaluation *EvaluationError
				require.ErrorAs(t, err, &evaluation)
			}
		})
	}
}

type requirePolicyStub struct {
	decision Decision
	err      error
}

func (s requirePolicyStub) Evaluate(context.Context, Subject) (Decision, error) {
	return s.decision, s.err
}
