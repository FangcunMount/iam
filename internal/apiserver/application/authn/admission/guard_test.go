package admission

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRequireMapsAdmissionDecisionToStableErrorCode(t *testing.T) {
	t.Parallel()

	subject := admissiondomain.Subject{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
	}
	tests := []struct {
		name      string
		decision  admissiondomain.Decision
		policyErr error
		wantCode  int
	}{
		{name: "admitted", decision: admissiondomain.Admit(subject)},
		{name: "identity missing", decision: admissiondomain.Deny(subject, admissiondomain.ReasonLoginIdentityMissing), wantCode: code.ErrLoginIdentityDisabled},
		{name: "identity disabled", decision: admissiondomain.Deny(subject, admissiondomain.ReasonLoginIdentityDisabled), wantCode: code.ErrLoginIdentityDisabled},
		{name: "identity owner mismatch", decision: admissiondomain.Deny(subject, admissiondomain.ReasonIdentityOwnerMismatch), wantCode: code.ErrUserBlocked},
		{name: "user missing", decision: admissiondomain.Deny(subject, admissiondomain.ReasonUserMissing), wantCode: code.ErrUserBlocked},
		{name: "user blocked", decision: admissiondomain.Deny(subject, admissiondomain.ReasonUserBlocked), wantCode: code.ErrUserBlocked},
		{name: "user inactive", decision: admissiondomain.Deny(subject, admissiondomain.ReasonUserInactive), wantCode: code.ErrUserInactive},
		{name: "policy failure", policyErr: errors.New("status unavailable"), wantCode: code.ErrInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policyStub{decision: tt.decision, err: tt.policyErr}

			err := Require(context.Background(), policy, subject)

			require.Equal(t, subject, policy.subject)
			if tt.wantCode == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.wantCode, perrors.ParseCoder(err).Code())
		})
	}
}

type policyStub struct {
	decision admissiondomain.Decision
	err      error
	subject  admissiondomain.Subject
}

func (s *policyStub) Evaluate(_ context.Context, subject admissiondomain.Subject) (admissiondomain.Decision, error) {
	s.subject = subject
	return s.decision, s.err
}
