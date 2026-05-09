package authn_test

import (
	"context"
	"testing"

	onboardingapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	credentialinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/credential"
	loginidentityinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/loginidentity"
	mysqlauthnuow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestOnboardingPersistsLoginIdentityAndPasswordCredentialV2(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(
		&mysqluser.UserPO{},
		&loginidentityinfra.PO{},
		&credentialinfra.V2PO{},
	))
	onboarder := onboardingapp.NewLoginIdentityOnboarder(mysqlauthnuow.NewUnitOfWork(db), onboardingPasswordHasherStub{}, nil, mysqluser.NewRepository(db), nil, nil)

	phone, err := meta.NewPhone("13811112222")
	require.NoError(t, err)
	email, err := meta.NewEmail("opera-loginidentity@example.com")
	require.NoError(t, err)
	password := "secret"
	tenantID := meta.FromUint64(9001)

	result, err := onboarder.Onboard(context.Background(), onboardingapp.OnboardingRequest{
		Scenario:       onboardingapp.OnboardOperaPassword,
		Name:           "zhangsan",
		Phone:          phone,
		Email:          email,
		OperaLoginID:   "zhangsan",
		ScopedTenantID: tenantID,
		Password:       &password,
	})
	require.NoError(t, err)
	require.False(t, result.LoginIdentityID.IsZero())
	require.False(t, result.CredentialID.IsZero())

	var identityCount int64
	require.NoError(t, db.Table("auth_login_identities").
		Where("id = ? AND provider = ? AND realm = ? AND identifier = ?", result.LoginIdentityID, "username", tenantID.String(), "zhangsan").
		Count(&identityCount).Error)
	require.Equal(t, int64(1), identityCount)

	var credentialCount int64
	require.NoError(t, db.Table("auth_credentials").
		Where("id = ? AND login_identity_id = ? AND type = ?", result.CredentialID, result.LoginIdentityID, "password").
		Count(&credentialCount).Error)
	require.Equal(t, int64(1), credentialCount)
}

func TestOnboardingPersistsPhoneLoginIdentityWithoutCredential(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(
		&mysqluser.UserPO{},
		&loginidentityinfra.PO{},
		&credentialinfra.V2PO{},
	))
	onboarder := onboardingapp.NewLoginIdentityOnboarder(mysqlauthnuow.NewUnitOfWork(db), onboardingPasswordHasherStub{}, nil, mysqluser.NewRepository(db), nil, nil)

	phone, err := meta.NewPhone("13811113333")
	require.NoError(t, err)
	email, err := meta.NewEmail("phone-loginidentity@example.com")
	require.NoError(t, err)

	result, err := onboarder.Onboard(context.Background(), onboardingapp.OnboardingRequest{
		Scenario: onboardingapp.OnboardPhoneOTP,
		Name:     "phone-user",
		Phone:    phone,
		Email:    email,
	})
	require.NoError(t, err)
	require.False(t, result.LoginIdentityID.IsZero())
	require.True(t, result.CredentialID.IsZero())

	var identityCount int64
	require.NoError(t, db.Table("auth_login_identities").
		Where("id = ? AND provider = ? AND realm = ? AND identifier = ?", result.LoginIdentityID, "phone", "global", phone.String()).
		Count(&identityCount).Error)
	require.Equal(t, int64(1), identityCount)

	var credentialCount int64
	require.NoError(t, db.Table("auth_credentials").Count(&credentialCount).Error)
	require.Equal(t, int64(0), credentialCount)
}

type onboardingPasswordHasherStub struct{}

func (onboardingPasswordHasherStub) Verify(string, string) bool { return true }
func (onboardingPasswordHasherStub) NeedRehash(string) bool     { return false }
func (onboardingPasswordHasherStub) Hash(plaintext string) (string, error) {
	return "hash:" + plaintext, nil
}
func (onboardingPasswordHasherStub) Pepper() string { return "pepper" }
