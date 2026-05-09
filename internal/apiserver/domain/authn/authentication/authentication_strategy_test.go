package authentication_test

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type hasherStub struct {
	pepper string
	need   bool
	newh   string
}

func (h *hasherStub) Verify(storedHash, plaintext string) bool { return storedHash == plaintext }
func (h *hasherStub) NeedRehash(storedHash string) bool        { return h.need }
func (h *hasherStub) Hash(plaintext string) (string, error)    { return h.newh, nil }
func (h *hasherStub) Pepper() string                           { return h.pepper }

func TestPasswordAuthStrategy_AllCases(t *testing.T) {
	ctx := context.Background()
	loginIdentityID := meta.FromUint64(12)
	userID := meta.FromUint64(22)
	tenantID := meta.FromUint64(1)

	makeLookup := func(status loginidentity.Status) *authentication.LoginIdentityLookup {
		return &authentication.LoginIdentityLookup{
			LoginIdentityID: loginIdentityID,
			UserID:          userID,
			Provider:        loginidentity.ProviderUsername,
			Realm:           tenantID.String(),
			Identifier:      "u",
			Status:          status,
			ScopedTenantID:  tenantID,
		}
	}
	makeAuth := func(identityRepo *loginIdentityRepoTestDouble, credRepo *loginIdentityCredentialRepoTestDouble, hasher *hasherStub) *authentication.Authenticator {
		return authentication.NewAuthenticator(authentication.NewPasswordAuthStrategyWithLoginIdentity(credRepo, identityRepo, hasher))
	}
	makeProof := func(username, password string, tenantID meta.ID) authentication.AuthCredential {
		proof, err := authentication.NewPasswordCredential(authentication.PasswordProofSpec{
			TenantID: tenantID,
			Username: username,
			Password: password,
		})
		require.NoError(t, err)
		return proof
	}

	credRepo := func(credentialID meta.ID, material string) *loginIdentityCredentialRepoTestDouble {
		return &loginIdentityCredentialRepoTestDouble{
			passwordByLoginIdentity: map[meta.ID]credentialMaterial{
				loginIdentityID: {credentialID: credentialID, material: material},
			},
		}
	}

	// 1. login identity not found -> invalid credential
	a1 := makeAuth(newLoginIdentityRepoTestDouble(), credRepo(meta.ZeroID, ""), &hasherStub{pepper: "p"})
	d1, err := a1.Authenticate(ctx, makeProof("u", "p", tenantID))
	require.NoError(t, err)
	require.False(t, d1.OK)
	require.Equal(t, code.ErrInvalidCredentials, d1.Code)

	// 2. disabled or locked identity
	a2 := makeAuth(newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusDisabled)), credRepo(meta.ZeroID, ""), &hasherStub{pepper: "p"})
	d2, err := a2.Authenticate(ctx, makeProof("u", "p", tenantID))
	require.NoError(t, err)
	require.False(t, d2.OK)
	require.Equal(t, code.ErrCredentialDisabled, d2.Code)

	lockedRepo := newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusActive))
	lockedRepo.lockedByID[loginIdentityID] = true
	a3 := makeAuth(lockedRepo, credRepo(meta.ZeroID, ""), &hasherStub{pepper: "p"})
	d3, err := a3.Authenticate(ctx, makeProof("u", "p", tenantID))
	require.NoError(t, err)
	require.False(t, d3.OK)
	require.Equal(t, code.ErrCredentialLocked, d3.Code)

	// 3. no password credential set
	a4 := makeAuth(newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusActive)), credRepo(meta.ZeroID, ""), &hasherStub{pepper: "p"})
	d4, err := a4.Authenticate(ctx, makeProof("u", "p", tenantID))
	require.NoError(t, err)
	require.False(t, d4.OK)
	require.Equal(t, code.ErrInvalidCredentials, d4.Code)

	// 4. wrong password -> invalid credential with CredentialID
	a5 := makeAuth(newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusActive)), credRepo(meta.FromUint64(100), "some-other"), &hasherStub{pepper: "p"})
	d5, err := a5.Authenticate(ctx, makeProof("u", "p", tenantID))
	require.NoError(t, err)
	require.False(t, d5.OK)
	require.Equal(t, code.ErrInvalidCredentials, d5.Code)
	require.Equal(t, meta.FromUint64(100), d5.CredentialID)

	// 5. success, need rehash -> ShouldRotate true and NewMaterial set
	pepper := "pep"
	pass := "pwd"
	stored := pass + pepper
	a6 := makeAuth(newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusActive)), credRepo(meta.FromUint64(200), stored), &hasherStub{pepper: pepper, need: true, newh: "new-hash"})
	d6, err := a6.Authenticate(ctx, makeProof("u", pass, tenantID))
	require.NoError(t, err)
	require.True(t, d6.OK)
	require.True(t, d6.ShouldRotate)
	require.Equal(t, []byte("new-hash"), d6.NewMaterial)

	// 6. success, no rehash
	a7 := makeAuth(newLoginIdentityRepoTestDouble(makeLookup(loginidentity.StatusActive)), credRepo(meta.FromUint64(200), stored), &hasherStub{pepper: pepper, need: false})
	d7, err := a7.Authenticate(ctx, makeProof("u", pass, tenantID))
	require.NoError(t, err)
	require.True(t, d7.OK)
	require.False(t, d7.ShouldRotate)

	// 7. mock-consumer maps to username/default and does not require tenant scope.
	mockIdentityID := meta.FromUint64(13)
	mockRepo := newLoginIdentityRepoTestDouble(&authentication.LoginIdentityLookup{
		LoginIdentityID: mockIdentityID,
		UserID:          meta.FromUint64(23),
		Provider:        loginidentity.ProviderUsername,
		Realm:           loginidentity.RealmDefault,
		Identifier:      "ref@example.com",
		Status:          loginidentity.StatusActive,
	})
	mockCreds := &loginIdentityCredentialRepoTestDouble{
		passwordByLoginIdentity: map[meta.ID]credentialMaterial{
			mockIdentityID: {credentialID: meta.FromUint64(201), material: stored},
		},
	}
	a8 := makeAuth(mockRepo, mockCreds, &hasherStub{pepper: pepper, need: false})
	d8, err := a8.Authenticate(ctx, makeProof("ref@example.com", pass, meta.ZeroID))
	require.NoError(t, err)
	require.True(t, d8.OK)
	require.NotNil(t, d8.Principal)
	require.Equal(t, mockIdentityID, d8.Principal.LoginIdentityID)
	require.Equal(t, meta.FromUint64(23), d8.Principal.UserID)
	require.Equal(t, loginidentity.RealmDefault, d8.Principal.Realm)
}
