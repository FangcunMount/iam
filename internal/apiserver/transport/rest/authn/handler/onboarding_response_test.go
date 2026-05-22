package handler

import (
	"testing"

	signupApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signup"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSignupResultToResponseUsesNullableCredential(t *testing.T) {
	t.Parallel()

	withoutCredential := signupResultToResponse(&signupApp.SignupResult{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
	})
	require.Nil(t, withoutCredential.Credential)

	withCredential := signupResultToResponse(&signupApp.SignupResult{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		Credential: &signupApp.SignupCredential{
			ID:   meta.FromUint64(3),
			Type: credDomain.CredPassword,
		},
	})
	require.NotNil(t, withCredential.Credential)
	require.Equal(t, uint64(3), withCredential.Credential.ID)
	require.Equal(t, "password", withCredential.Credential.Type)
}
