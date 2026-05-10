package credential_test

import (
	"errors"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPasswordIssuerIssuesHashedPasswordCredential(t *testing.T) {
	t.Parallel()

	issuer := credential.NewPasswordIssuer(passwordHasherStub{
		hash:   "$argon2id$hash",
		pepper: "-pepper",
	})

	cred, err := issuer.IssuePassword(credential.IssuePasswordRequest{
		LoginIdentityID: meta.ID(1),
		PlainPassword:   "secret",
	})

	require.NoError(t, err)
	require.Equal(t, meta.ID(1), cred.LoginIdentityID)
	require.Equal(t, credential.CredPassword, cred.Type)
	require.Equal(t, []byte("$argon2id$hash"), cred.Material)
	require.NotNil(t, cred.Algo)
	require.Equal(t, "argon2id", *cred.Algo)
}

func TestPasswordIssuerAcceptsAlreadyHashedPassword(t *testing.T) {
	t.Parallel()

	issuer := credential.NewPasswordIssuer(passwordHasherStub{})

	cred, err := issuer.IssuePassword(credential.IssuePasswordRequest{
		LoginIdentityID: meta.ID(1),
		HashedPassword:  "$bcrypt$hash",
		Algo:            "bcrypt",
	})

	require.NoError(t, err)
	require.Equal(t, []byte("$bcrypt$hash"), cred.Material)
	require.NotNil(t, cred.Algo)
	require.Equal(t, "bcrypt", *cred.Algo)
}

func TestPasswordIssuerValidatesPasswordMaterial(t *testing.T) {
	t.Parallel()

	issuer := credential.NewPasswordIssuer(passwordHasherStub{})

	_, err := issuer.IssuePassword(credential.IssuePasswordRequest{})
	require.Error(t, err)

	_, err = issuer.IssuePassword(credential.IssuePasswordRequest{
		LoginIdentityID: meta.ID(1),
	})
	require.Error(t, err)

	_, err = issuer.IssuePassword(credential.IssuePasswordRequest{
		LoginIdentityID: meta.ID(1),
		HashedPassword:  "$hash",
	})
	require.Error(t, err)
}

func TestPasswordIssuerReturnsHashError(t *testing.T) {
	t.Parallel()

	issuer := credential.NewPasswordIssuer(passwordHasherStub{err: errors.New("hash failed")})

	_, err := issuer.IssuePassword(credential.IssuePasswordRequest{
		LoginIdentityID: meta.ID(1),
		PlainPassword:   "secret",
	})

	require.Error(t, err)
}

type passwordHasherStub struct {
	hash   string
	pepper string
	err    error
}

func (s passwordHasherStub) Hash(plaintext string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.hash, nil
}

func (s passwordHasherStub) Verify(string, string) bool {
	return false
}

func (s passwordHasherStub) Pepper() string {
	return s.pepper
}

func (s passwordHasherStub) NeedRehash(string) bool {
	return false
}
