package credential

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
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

func (s *credentialRepoStub) UpdateMaterial(_ context.Context, id meta.ID, material []byte, _ string) error {
	s.material = append([]byte(nil), material...)
	if cred := s.items[id]; cred != nil {
		cred.Material = append([]byte(nil), material...)
	}
	return nil
}

func (s *credentialRepoStub) UpdateStatus(context.Context, meta.ID, credDomain.CredentialStatus) error {
	return nil
}

func (s *credentialRepoStub) UpdateAuthState(context.Context, *credDomain.Credential) error {
	return nil
}

func (s *credentialRepoStub) GetByID(_ context.Context, id meta.ID) (*credDomain.Credential, error) {
	return s.items[id], nil
}

func (s *credentialRepoStub) GetByLoginIdentityIDAndType(context.Context, meta.ID, credDomain.CredentialType) (*credDomain.Credential, error) {
	return nil, nil
}
