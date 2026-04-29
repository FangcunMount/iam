package authentication_test

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// local stubs
type accRepoStub struct {
	accountID      meta.ID
	userID         meta.ID
	accountType    string
	scopedTenantID meta.ID
	enabled        bool
	locked         bool
	err            error
}

func (s *accRepoStub) FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*authentication.UsernameLoginLookup, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.accountID.IsZero() {
		return nil, nil
	}
	typ := s.accountType
	if typ == "" {
		typ = "wc-minip"
	}
	return &authentication.UsernameLoginLookup{
		AccountID:      s.accountID,
		UserID:         s.userID,
		AccountType:    typ,
		ScopedTenantID: s.scopedTenantID,
	}, nil
}
func (s *accRepoStub) GetAccountStatus(ctx context.Context, accountID meta.ID) (bool, bool, error) {
	return s.enabled, s.locked, s.err
}

type credRepoStub struct {
	credID meta.ID
	stored string
	err    error
}

func (s *credRepoStub) FindPasswordCredential(ctx context.Context, accountID meta.ID) (meta.ID, string, error) {
	return s.credID, s.stored, s.err
}
func (s *credRepoStub) FindPhoneOTPCredential(ctx context.Context, phoneE164 string) (meta.ID, meta.ID, meta.ID, error) {
	return 0, 0, 0, nil
}
func (s *credRepoStub) FindOAuthCredential(ctx context.Context, idpType, appID, idpIdentifier string) (meta.ID, meta.ID, meta.ID, error) {
	return 0, 0, 0, nil
}

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

	// helper to build Authenticator
	makeAuth := func(acc *accRepoStub, cred *credRepoStub, hasher *hasherStub) *authentication.Authenticator {
		return authentication.NewAuthenticator(cred, acc, hasher, nil, nil)
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

	// 1. account not found -> invalid credential
	acc1 := &accRepoStub{accountID: 0}
	cred1 := &credRepoStub{}
	hasher1 := &hasherStub{pepper: "p"}
	a1 := makeAuth(acc1, cred1, hasher1)
	d1, err := a1.Authenticate(ctx, makeProof("u", "p", meta.ID(1)))
	require.NoError(t, err)
	require.False(t, d1.OK)
	require.Equal(t, authentication.ErrInvalidCredential, d1.ErrCode)

	// 2. disabled or locked
	acc2 := &accRepoStub{accountID: meta.ID(10), userID: meta.ID(20), enabled: false}
	a2 := makeAuth(acc2, cred1, hasher1)
	d2, err := a2.Authenticate(ctx, makeProof("u", "p", meta.ID(1)))
	require.NoError(t, err)
	require.False(t, d2.OK)
	require.Equal(t, authentication.ErrDisabled, d2.ErrCode)

	acc3 := &accRepoStub{accountID: meta.ID(11), userID: meta.ID(21), enabled: true, locked: true}
	a3 := makeAuth(acc3, cred1, hasher1)
	d3, err := a3.Authenticate(ctx, makeProof("u", "p", meta.ID(1)))
	require.NoError(t, err)
	require.False(t, d3.OK)
	require.Equal(t, authentication.ErrLocked, d3.ErrCode)

	// 3. no credential set
	acc4 := &accRepoStub{accountID: meta.ID(12), userID: meta.ID(22), enabled: true}
	cred4 := &credRepoStub{credID: 0}
	a4 := makeAuth(acc4, cred4, hasher1)
	d4, err := a4.Authenticate(ctx, makeProof("u", "p", meta.ID(1)))
	require.NoError(t, err)
	require.False(t, d4.OK)
	require.Equal(t, authentication.ErrInvalidCredential, d4.ErrCode)

	// 4. wrong password -> invalid credential with CredentialID
	storedWrong := "some-other"
	cred5 := &credRepoStub{credID: meta.ID(100), stored: storedWrong}
	a5 := makeAuth(acc4, cred5, hasher1)
	d5, err := a5.Authenticate(ctx, makeProof("u", "p", meta.ID(1)))
	require.NoError(t, err)
	require.False(t, d5.OK)
	require.Equal(t, authentication.ErrInvalidCredential, d5.ErrCode)
	require.Equal(t, meta.ID(100), d5.CredentialID)

	// 5. success, need rehash -> ShouldRotate true and NewMaterial set
	pepper := "pep"
	pass := "pwd"
	// stored hash == plaintextWithPepper
	stored := pass + pepper
	cred6 := &credRepoStub{credID: meta.ID(200), stored: stored}
	hasher6 := &hasherStub{pepper: pepper, need: true, newh: "new-hash"}
	a6 := makeAuth(acc4, cred6, hasher6)
	d6, err := a6.Authenticate(ctx, makeProof("u", pass, meta.ID(1)))
	require.NoError(t, err)
	require.True(t, d6.OK)
	require.True(t, d6.ShouldRotate)
	require.Equal(t, []byte("new-hash"), d6.NewMaterial)

	// 6. success, no rehash
	hasher7 := &hasherStub{pepper: pepper, need: false}
	a7 := makeAuth(acc4, cred6, hasher7)
	d7, err := a7.Authenticate(ctx, makeProof("u", pass, meta.ID(1)))
	require.NoError(t, err)
	require.True(t, d7.OK)
	require.False(t, d7.ShouldRotate)

	// 7. mock-consumer should follow non-opera branch and not require tenant scope
	acc8 := &accRepoStub{
		accountID:   meta.ID(13),
		userID:      meta.ID(23),
		accountType: "mock-consumer",
		enabled:     true,
	}
	a8 := makeAuth(acc8, cred6, hasher7)
	d8, err := a8.Authenticate(ctx, makeProof("ref@example.com", pass, meta.ID(0)))
	require.NoError(t, err)
	require.True(t, d8.OK)
	require.NotNil(t, d8.Principal)
	require.Equal(t, meta.ID(13), d8.Principal.AccountID)
	require.Equal(t, meta.ID(23), d8.Principal.UserID)
}
