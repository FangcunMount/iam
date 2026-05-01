package credential

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	accountpo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/account"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 编译时验证接口实现
var _ credDomain.Repository = (*Repository)(nil)

func ExampleNewRepository() {
	var db *gorm.DB
	repo := NewRepository(db)
	_ = repo
}

func TestCredentialRepositoryMissingWritesTranslateToBusinessNotFound(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&PO{}))

	repo := NewRepository(db)
	missingID := meta.FromUint64(404)
	now := time.Now()

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "material",
			run: func(ctx context.Context) error {
				return repo.UpdateMaterial(ctx, missingID, []byte("hash"), "argon2id")
			},
		},
		{
			name: "status",
			run: func(ctx context.Context) error {
				return repo.UpdateStatus(ctx, missingID, credDomain.CredStatusDisabled)
			},
		},
		{
			name: "failed attempts",
			run: func(ctx context.Context) error {
				return repo.UpdateFailedAttempts(ctx, missingID, 3)
			},
		},
		{
			name: "locked until",
			run: func(ctx context.Context) error {
				return repo.UpdateLockedUntil(ctx, missingID, &now)
			},
		},
		{
			name: "last success",
			run: func(ctx context.Context) error {
				return repo.UpdateLastSuccessAt(ctx, missingID, now)
			},
		},
		{
			name: "last failure",
			run: func(ctx context.Context) error {
				return repo.UpdateLastFailureAt(ctx, missingID, now)
			},
		},
		{
			name: "delete",
			run: func(ctx context.Context) error {
				return repo.Delete(ctx, missingID)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(context.Background())
			require.Error(t, err)
			require.True(t, perrors.IsCode(err, code.ErrCredentialNotFound), "err=%v", err)
		})
	}
}

func TestCredentialRepositoryFindPhoneOTPMissingTranslatesToBusinessNotFound(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&PO{}, &accountpo.AccountPO{}))

	repo := NewRepository(db)
	_, _, _, err := repo.FindPhoneOTPCredential(context.Background(), "+8613800138000")

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrCredentialNotFound), "err=%v", err)
}
