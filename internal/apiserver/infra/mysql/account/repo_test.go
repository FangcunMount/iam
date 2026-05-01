package account

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 编译时验证接口实现。
var _ account.Repository = (*AccountRepository)(nil)

func ExampleNewAccountRepository() {
	var db *gorm.DB
	repo := NewAccountRepository(db)
	_ = repo
}

func TestAccountRepositoryUpdateMissingTranslatesToBusinessNotFound(t *testing.T) {
	t.Parallel()

	db := testhelpers.SetupTempSQLiteDB(t)
	require.NoError(t, db.AutoMigrate(&AccountPO{}))

	repo := NewAccountRepository(db)
	missingID := meta.FromUint64(404)

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "unique id",
			run: func(ctx context.Context) error {
				return repo.UpdateUniqueID(ctx, missingID, account.UnionID("union-id"))
			},
		},
		{
			name: "status",
			run: func(ctx context.Context) error {
				return repo.UpdateStatus(ctx, missingID, account.StatusDisabled)
			},
		},
		{
			name: "profile",
			run: func(ctx context.Context) error {
				return repo.UpdateProfile(ctx, missingID, map[string]string{"name": "alice"})
			},
		},
		{
			name: "meta",
			run: func(ctx context.Context) error {
				return repo.UpdateMeta(ctx, missingID, map[string]string{"source": "test"})
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(context.Background())
			require.Error(t, err)
			require.True(t, perrors.IsCode(err, code.ErrNotFoundAccount), "err=%v", err)
		})
	}
}
