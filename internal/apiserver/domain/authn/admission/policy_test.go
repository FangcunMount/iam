package admission

import (
	"context"
	"errors"
	"testing"

	loginidentitydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPolicyEvaluatesAdmissionDecisionTable(t *testing.T) {
	t.Parallel()

	subject := Subject{
		UserID:          meta.FromUint64(1001),
		LoginIdentityID: meta.FromUint64(2001),
	}
	activeIdentity := func(userID meta.ID) *loginidentitydomain.LoginIdentity {
		return &loginidentitydomain.LoginIdentity{
			ID:         subject.LoginIdentityID,
			UserID:     userID,
			Provider:   loginidentitydomain.ProviderUsername,
			Realm:      loginidentitydomain.RealmDefault,
			Identifier: "zhangsan",
			Status:     loginidentitydomain.StatusActive,
		}
	}

	tests := []struct {
		name          string
		identity      *loginidentitydomain.LoginIdentity
		userStatus    useraccess.Status
		wantOutcome   Outcome
		wantReason    DenialReason
		wantUserReads int
	}{
		{
			name:          "active identity owned by active user is admitted",
			identity:      activeIdentity(subject.UserID),
			userStatus:    useraccess.StatusActive,
			wantOutcome:   OutcomeAdmitted,
			wantUserReads: 1,
		},
		{
			name:        "missing login identity is denied before reading user",
			wantOutcome: OutcomeDenied,
			wantReason:  ReasonLoginIdentityMissing,
		},
		{
			name:        "identity owned by another user is denied before reading user",
			identity:    activeIdentity(meta.FromUint64(1002)),
			wantOutcome: OutcomeDenied,
			wantReason:  ReasonIdentityOwnerMismatch,
		},
		{
			name: "disabled login identity is denied before reading user",
			identity: &loginidentitydomain.LoginIdentity{
				ID:     subject.LoginIdentityID,
				UserID: subject.UserID,
				Status: loginidentitydomain.StatusDisabled,
			},
			wantOutcome: OutcomeDenied,
			wantReason:  ReasonLoginIdentityDisabled,
		},
		{
			name:          "missing user is denied",
			identity:      activeIdentity(subject.UserID),
			userStatus:    useraccess.StatusMissing,
			wantOutcome:   OutcomeDenied,
			wantReason:    ReasonUserMissing,
			wantUserReads: 1,
		},
		{
			name:          "blocked user is denied",
			identity:      activeIdentity(subject.UserID),
			userStatus:    useraccess.StatusBlocked,
			wantOutcome:   OutcomeDenied,
			wantReason:    ReasonUserBlocked,
			wantUserReads: 1,
		},
		{
			name:          "inactive user is denied",
			identity:      activeIdentity(subject.UserID),
			userStatus:    useraccess.StatusInactive,
			wantOutcome:   OutcomeDenied,
			wantReason:    ReasonUserInactive,
			wantUserReads: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &userStatusReaderStub{status: tt.userStatus}
			identities := &loginIdentityReaderStub{identity: tt.identity}

			decision, err := NewPolicy(users, identities).Evaluate(context.Background(), subject)

			require.NoError(t, err)
			require.Equal(t, subject, decision.Subject)
			require.Equal(t, tt.wantOutcome, decision.Outcome)
			require.Equal(t, tt.wantReason, decision.Reason)
			require.Equal(t, tt.wantOutcome == OutcomeAdmitted, decision.IsAdmitted())
			require.Equal(t, tt.wantUserReads, users.calls)
		})
	}
}

func TestPolicyReturnsTechnicalErrorsSeparatelyFromDenial(t *testing.T) {
	t.Parallel()

	subject := Subject{UserID: meta.FromUint64(1001), LoginIdentityID: meta.FromUint64(2001)}
	activeIdentity := &loginidentitydomain.LoginIdentity{
		ID:     subject.LoginIdentityID,
		UserID: subject.UserID,
		Status: loginidentitydomain.StatusActive,
	}

	tests := []struct {
		name     string
		users    useraccess.UserStatusReader
		identity LoginIdentityReader
		want     string
	}{
		{
			name:  "login identity reader is required",
			users: &userStatusReaderStub{status: useraccess.StatusActive},
			want:  "login identity reader is not configured",
		},
		{
			name:     "login identity read failure",
			users:    &userStatusReaderStub{status: useraccess.StatusActive},
			identity: &loginIdentityReaderStub{err: errors.New("database unavailable")},
			want:     "load login identity status",
		},
		{
			name:     "user status reader is required",
			identity: &loginIdentityReaderStub{identity: activeIdentity},
			want:     "identity user status reader is not configured",
		},
		{
			name:     "user status read failure",
			users:    &userStatusReaderStub{err: errors.New("identity unavailable")},
			identity: &loginIdentityReaderStub{identity: activeIdentity},
			want:     "load user status",
		},
		{
			name:     "unknown user status",
			users:    &userStatusReaderStub{status: useraccess.Status("suspended")},
			identity: &loginIdentityReaderStub{identity: activeIdentity},
			want:     "unknown user status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := NewPolicy(tt.users, tt.identity).Evaluate(context.Background(), subject)

			require.ErrorContains(t, err, tt.want)
			require.Equal(t, Decision{}, decision)
		})
	}
}

type userStatusReaderStub struct {
	status useraccess.Status
	err    error
	calls  int
}

func (s *userStatusReaderStub) ReadUserStatus(context.Context, meta.ID) (useraccess.Status, error) {
	s.calls++
	return s.status, s.err
}

type loginIdentityReaderStub struct {
	identity *loginidentitydomain.LoginIdentity
	err      error
}

func (s *loginIdentityReaderStub) GetByID(context.Context, meta.ID) (*loginidentitydomain.LoginIdentity, error) {
	return s.identity, s.err
}
