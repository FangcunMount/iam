package admission

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestRequireMapsAdmissionDecisionToStableErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		status   admissiondomain.Status
		policyErr error
		wantCode int
	}{
		{name: "active", status: admissiondomain.StatusActive},
		{name: "blocked", status: admissiondomain.StatusBlocked, wantCode: code.ErrUserBlocked},
		{name: "disabled", status: admissiondomain.StatusDisabled, wantCode: code.ErrLoginIdentityDisabled},
		{name: "inactive", status: admissiondomain.StatusInactive, wantCode: code.ErrUserInactive},
		{name: "policy failure", policyErr: errors.New("status unavailable"), wantCode: code.ErrInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Require(context.Background(), policyStub{status: tt.status, err: tt.policyErr}, meta.FromUint64(1), meta.FromUint64(2))
			if tt.wantCode == 0 {
				if err != nil {
					t.Fatalf("Require() error = %v", err)
				}
				return
			}
			if got := perrors.ParseCoder(err).Code(); got != tt.wantCode {
				t.Fatalf("Require() code = %d, want %d, err = %v", got, tt.wantCode, err)
			}
		})
	}
}

type policyStub struct {
	status admissiondomain.Status
	err    error
}

func (s policyStub) Evaluate(context.Context, meta.ID, meta.ID) (admissiondomain.Decision, error) {
	return admissiondomain.Decision{Status: s.status}, s.err
}
