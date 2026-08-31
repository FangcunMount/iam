package admission

import (
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMapErrorMapsAdmissionDomainErrorsToStableErrorCode(t *testing.T) {
	t.Parallel()

	subject := admissiondomain.Subject{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
	}
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{name: "nil"},
		{name: "identity missing", err: deniedError(subject, admissiondomain.ReasonLoginIdentityMissing), wantCode: code.ErrLoginIdentityDisabled},
		{name: "identity disabled", err: deniedError(subject, admissiondomain.ReasonLoginIdentityDisabled), wantCode: code.ErrLoginIdentityDisabled},
		{name: "identity owner mismatch", err: deniedError(subject, admissiondomain.ReasonIdentityOwnerMismatch), wantCode: code.ErrUserBlocked},
		{name: "user missing", err: deniedError(subject, admissiondomain.ReasonUserMissing), wantCode: code.ErrUserBlocked},
		{name: "user blocked", err: deniedError(subject, admissiondomain.ReasonUserBlocked), wantCode: code.ErrUserBlocked},
		{name: "user inactive", err: deniedError(subject, admissiondomain.ReasonUserInactive), wantCode: code.ErrUserInactive},
		{name: "policy failure", err: &admissiondomain.EvaluationError{Err: errors.New("status unavailable")}, wantCode: code.ErrInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapError(tt.err)
			if tt.wantCode == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Equal(t, tt.wantCode, perrors.ParseCoder(err).Code())
		})
	}
}

func TestMapErrorPreservesNonAdmissionError(t *testing.T) {
	t.Parallel()
	want := errors.New("token store unavailable")
	require.ErrorIs(t, MapError(want), want)
}

func deniedError(subject admissiondomain.Subject, reason admissiondomain.DenialReason) error {
	return &admissiondomain.DeniedError{Decision: admissiondomain.Deny(subject, reason)}
}
