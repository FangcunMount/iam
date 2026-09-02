package credential

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type issuerHasherStub struct {
	pepper      string
	hash        string
	returnEmpty bool
	err         error
}

func (h *issuerHasherStub) Hash(plaintext string) (string, error) {
	if h.err != nil {
		return "", h.err
	}
	if h.returnEmpty {
		return "", nil
	}
	if h.hash != "" {
		return h.hash, nil
	}
	return "hashed:" + plaintext, nil
}

func (h *issuerHasherStub) Pepper() string { return h.pepper }

func TestIssuePasswordCredentialRequiresLoginIdentity(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{})
	_, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{})
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestIssuePasswordCredentialHashesPlainPasswordWithPepper(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{pepper: "pep"})
	cred, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{
		LoginIdentityID: meta.FromUint64(10),
		PlainPassword:   "secret",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("hashed:secretpep"), cred.Material)
	require.NotNil(t, cred.Algo)
	require.Equal(t, "argon2id", *cred.Algo)
}

func TestIssuePasswordCredentialAcceptsPrehashedMaterial(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{})
	cred, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{
		LoginIdentityID: meta.FromUint64(11),
		HashedPassword:  "phc$argon2id$v=19$m=65536,t=3,p=4$...",
		Algo:            "argon2id",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("phc$argon2id$v=19$m=65536,t=3,p=4$..."), cred.Material)
	require.Equal(t, "argon2id", *cred.Algo)
}

func TestIssuePasswordCredentialRequiresAlgoForPrehashedMaterial(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{})
	_, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{
		LoginIdentityID: meta.FromUint64(12),
		HashedPassword:  "phc$argon2id$v=19$m=65536,t=3,p=4$...",
	})
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidArgument, perrors.ParseCoder(err).Code())
}

func TestIssuePasswordCredentialRejectsEmptyHashedMaterial(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{returnEmpty: true})
	_, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{
		LoginIdentityID: meta.FromUint64(13),
		PlainPassword:   "secret",
	})
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidCredential, perrors.ParseCoder(err).Code())
}

func TestIssuePasswordCredentialUsesActualHasherAlgorithmForPlaintext(t *testing.T) {
	t.Parallel()

	issuer := NewPasswordIssuer(&issuerHasherStub{})
	cred, err := issuer.IssuePasswordCredential(PasswordCredentialRequest{
		LoginIdentityID: meta.FromUint64(14),
		PlainPassword:   "secret",
		Algo:            "bcrypt",
	})
	require.NoError(t, err)
	require.NotNil(t, cred.Algo)
	require.Equal(t, "argon2id", *cred.Algo)
}
