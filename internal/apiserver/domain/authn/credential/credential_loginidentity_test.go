package credential

import (
	"testing"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordCredential(t *testing.T) {
	loginIdentityID := meta.FromUint64(2001)

	cred := NewPasswordCredential(loginIdentityID, []byte("hash"), "argon2id")

	require.Equal(t, loginIdentityID, cred.LoginIdentityID)
	require.Equal(t, loginIdentityID, cred.LoginIdentityID)
	require.Equal(t, CredPassword, cred.Type)
	require.True(t, cred.IsPasswordType())
}

func TestBinderCanBindPasswordToLoginIdentity(t *testing.T) {
	algo := "argon2id"
	loginIdentityID := meta.FromUint64(2001)

	cred, err := NewBinder().Bind(BindSpec{
		LoginIdentityID: loginIdentityID,
		Type:            CredPassword,
		Material:        []byte("hash"),
		Algo:            &algo,
	})

	require.NoError(t, err)
	require.Equal(t, loginIdentityID, cred.LoginIdentityID)
	require.Equal(t, loginIdentityID, cred.LoginIdentityID)
	require.Equal(t, CredPassword, cred.Type)
}
