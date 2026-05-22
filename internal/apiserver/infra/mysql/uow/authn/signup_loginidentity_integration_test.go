package authn_test

import (
	"context"
	"testing"

	signupapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signup"
	credentialinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/credential"
	loginidentityinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/loginidentity"
	mysqlauthnuow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/authn"
	mysqluser "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSignupPersistsLoginIdentityAndPasswordCredentialV2(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(
		&mysqluser.UserPO{},
		&loginidentityinfra.PO{},
		&credentialinfra.V2PO{},
	))
	svc := signupapp.NewSignupService(mysqlauthnuow.NewUnitOfWork(db), signupPasswordHasherStub{}, nil, mysqluser.NewRepository(db), nil, nil)

	phone, err := meta.NewPhone("13811112222")
	require.NoError(t, err)
	email, err := meta.NewEmail("opera-loginidentity@example.com")
	require.NoError(t, err)
	password := "secret"
	tenantID := meta.FromUint64(9001)

	result, err := svc.SignUp(context.Background(), signupapp.SignupRequest{
		User: signupapp.SignupUserInput{
			Name:  "zhangsan",
			Phone: phone,
			Email: email,
		},
		LoginIdentity: signupapp.UsernameLoginIdentityInput{
			Username:      "zhangsan",
			RealmTenantID: tenantID,
		},
		Credential: &signupapp.SignupCredentialInput{
			Password: &signupapp.PasswordCredentialInput{Plaintext: password},
		},
	})
	require.NoError(t, err)
	require.False(t, result.LoginIdentityID.IsZero())
	require.NotNil(t, result.Credential)
	require.False(t, result.Credential.ID.IsZero())

	var identityCount int64
	require.NoError(t, db.Table("auth_login_identities").
		Where("id = ? AND provider = ? AND realm = ? AND identifier = ?", result.LoginIdentityID, "username", tenantID.String(), "zhangsan").
		Count(&identityCount).Error)
	require.Equal(t, int64(1), identityCount)

	var credentialCount int64
	require.NoError(t, db.Table("auth_credentials").
		Where("id = ? AND login_identity_id = ? AND type = ?", result.Credential.ID, result.LoginIdentityID, "password").
		Count(&credentialCount).Error)
	require.Equal(t, int64(1), credentialCount)
}

func TestSignupPersistsWechatMiniLoginIdentityWithoutCredential(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(
		&mysqluser.UserPO{},
		&loginidentityinfra.PO{},
		&credentialinfra.V2PO{},
	))
	svc := signupapp.NewSignupService(mysqlauthnuow.NewUnitOfWork(db), signupPasswordHasherStub{}, nil, mysqluser.NewRepository(db), nil, nil)

	phone, err := meta.NewPhone("13811113333")
	require.NoError(t, err)
	email, err := meta.NewEmail("wechat-loginidentity@example.com")
	require.NoError(t, err)
	appID := "wx-app"
	openID := "openid-1"
	unionID := "union-1"

	result, err := svc.SignUp(context.Background(), signupapp.SignupRequest{
		User: signupapp.SignupUserInput{
			Name:  "wechat-user",
			Phone: phone,
			Email: email,
		},
		LoginIdentity: signupapp.WechatMiniLoginIdentityInput{
			AppID:   &appID,
			OpenID:  &openID,
			UnionID: &unionID,
		},
	})
	require.NoError(t, err)
	require.False(t, result.LoginIdentityID.IsZero())
	require.Nil(t, result.Credential)

	var identityCount int64
	require.NoError(t, db.Table("auth_login_identities").
		Where("id = ? AND provider = ? AND realm = ? AND identifier = ? AND global_identifier = ?", result.LoginIdentityID, "wechat_minip", appID, openID, unionID).
		Count(&identityCount).Error)
	require.Equal(t, int64(1), identityCount)

	var credentialCount int64
	require.NoError(t, db.Table("auth_credentials").Count(&credentialCount).Error)
	require.Equal(t, int64(0), credentialCount)
}

type signupPasswordHasherStub struct{}

func (signupPasswordHasherStub) Verify(string, string) bool { return true }
func (signupPasswordHasherStub) NeedRehash(string) bool     { return false }
func (signupPasswordHasherStub) Hash(plaintext string) (string, error) {
	return "hash:" + plaintext, nil
}
func (signupPasswordHasherStub) Pepper() string { return "pepper" }
