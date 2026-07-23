package credential

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestRecorderRecordsFailureSuccessAndRotation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	repo := newCredentialRepoStub()
	cred := credDomain.NewPasswordCredential(meta.FromUint64(10), []byte("old"), "argon2id")
	cred.ID = meta.FromUint64(100)
	repo.items[cred.ID] = cred
	rec := NewRecorder(Dependencies{Credentials: repo, Now: func() time.Time { return now }})

	err := rec.Record(context.Background(), authentication.AuthDecision{
		OK:           false,
		Code:         code.ErrInvalidCredentials,
		CredentialID: cred.ID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, cred.FailedAttempts)
	require.NotNil(t, cred.LastFailureAt)

	err = rec.Record(context.Background(), authentication.AuthDecision{
		OK:           true,
		CredentialID: cred.ID,
		ShouldRotate: true,
		NewMaterial:  []byte("new"),
	})
	require.NoError(t, err)
	require.Equal(t, 0, cred.FailedAttempts)
	require.NotNil(t, cred.LastSuccessAt)
	require.Equal(t, []byte("new"), repo.material)
}

type credentialRepoStub struct {
	items    map[meta.ID]*credDomain.Credential
	material []byte
}

func newCredentialRepoStub() *credentialRepoStub {
	return &credentialRepoStub{items: map[meta.ID]*credDomain.Credential{}}
}

func (s *credentialRepoStub) Create(context.Context, *credDomain.Credential) error {
	return nil
}

func (s *credentialRepoStub) UpdateStatus(context.Context, meta.ID, credDomain.CredentialStatus) error {
	return nil
}

func (s *credentialRepoStub) RecordAuthenticationFailure(_ context.Context, id meta.ID, now time.Time, policy credDomain.LockoutPolicy) (credDomain.AuthenticationState, error) {
	cred := s.items[id]
	if cred == nil {
		return credDomain.AuthenticationState{}, nil
	}
	cred.RecordFailure(now)
	newlyLocked := cred.ApplyLockPolicy(now, policy)
	return credDomain.AuthenticationState{
		FailedAttempts: cred.FailedAttempts,
		LockedUntil:    cred.LockedUntil,
		NewlyLocked:    newlyLocked,
	}, nil
}

func (s *credentialRepoStub) RecordAuthenticationSuccess(_ context.Context, id meta.ID, now time.Time, rotation *credDomain.MaterialRotation) error {
	cred := s.items[id]
	if cred == nil {
		return nil
	}
	cred.RecordSuccess(now)
	if rotation != nil {
		s.material = append([]byte(nil), rotation.Material...)
		cred.RotateMaterial(rotation.Material, rotation.Algo)
	}
	return nil
}

func (s *credentialRepoStub) GetByID(_ context.Context, id meta.ID) (*credDomain.Credential, error) {
	return s.items[id], nil
}

func (s *credentialRepoStub) GetByLoginIdentityIDAndType(context.Context, meta.ID, credDomain.CredentialType) (*credDomain.Credential, error) {
	return nil, nil
}
